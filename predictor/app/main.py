"""Predictor service entrypoint and deterministic risk scoring logic.

This module hosts the FastAPI application used by KubeLens to score pod and
node operational risk. Scoring is intentionally deterministic and based on
current cluster snapshots and recent warning events, so UI behavior remains
stable and explainable.

Environment variables:
    OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: OTLP traces endpoint.
    OTEL_EXPORTER_OTLP_ENDPOINT: Fallback OTLP endpoint.
    OTEL_EXPORTER_OTLP_TRACES_PROTOCOL: "grpc" or "http/protobuf".
    OTEL_EXPORTER_OTLP_PROTOCOL: Fallback OTLP protocol.
    OTEL_EXPORTER_OTLP_TRACES_INSECURE: Whether OTLP exporter is insecure.
    OTEL_EXPORTER_OTLP_INSECURE: Fallback insecure toggle.
    OTEL_SERVICE_NAME: Service name for trace resource attributes.
    OTEL_TRACES_SAMPLE_RATIO: Trace sampling ratio in [0.0, 1.0].
    PREDICTOR_SHARED_SECRET: Optional shared secret for /predict requests.
    PREDICTOR_MODEL_PATH: Optional joblib model used for pod score blending.
"""

from __future__ import annotations

import logging
import os
from datetime import datetime, timezone

from fastapi import Depends, FastAPI, Header, HTTPException, status
from opentelemetry import trace
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.propagate import set_global_textmap
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.trace.sampling import ParentBased, TraceIdRatioBased
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator
from pydantic import BaseModel, Field

api = FastAPI(title="k8s-ops-predictor", version="1.0.0")
logger = logging.getLogger("predictor.telemetry")
_ml_model_cache_path: str | None = None
_ml_model_cache: object | None = None


def configure_telemetry(app: FastAPI) -> None:
    """Configure OpenTelemetry tracing for the FastAPI application.

    The function is defensive by design: when telemetry setup fails, startup
    continues and the predictor remains available without tracing.

    Args:
        app: FastAPI application instance to instrument.
    """

    endpoint = os.getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") or os.getenv(
        "OTEL_EXPORTER_OTLP_ENDPOINT", ""
    )
    endpoint = endpoint.strip()
    if endpoint == "":
        return

    protocol = os.getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL") or os.getenv(
        "OTEL_EXPORTER_OTLP_PROTOCOL", ""
    )
    protocol = protocol.strip().lower() or "grpc"

    insecure_raw = os.getenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE") or os.getenv(
        "OTEL_EXPORTER_OTLP_INSECURE", "true"
    )
    insecure = insecure_raw.strip().lower() in {"1", "true", "yes", "on"}

    service_name = os.getenv("OTEL_SERVICE_NAME", "k8s-ops-predictor").strip() or "k8s-ops-predictor"
    sample_raw = os.getenv("OTEL_TRACES_SAMPLE_RATIO", "1.0").strip()
    try:
        sample_ratio = float(sample_raw)
    except ValueError:
        sample_ratio = 1.0
    sample_ratio = max(0.0, min(1.0, sample_ratio))

    try:
        resource = Resource.create({"service.name": service_name})
        provider = TracerProvider(resource=resource, sampler=ParentBased(TraceIdRatioBased(sample_ratio)))

        if protocol in {"http", "http/protobuf"}:
            from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter as OTLPHTTPSpanExporter

            exporter = OTLPHTTPSpanExporter(endpoint=endpoint)
        else:
            from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter as OTLPGRPCSpanExporter

            exporter = OTLPGRPCSpanExporter(endpoint=endpoint, insecure=insecure)

        provider.add_span_processor(BatchSpanProcessor(exporter))
        trace.set_tracer_provider(provider)
        set_global_textmap(TraceContextTextMapPropagator())
        FastAPIInstrumentor.instrument_app(app)
    except Exception as exc:  # pragma: no cover - defensive startup path
        logger.warning("OpenTelemetry initialization failed; continuing without tracing: %s", exc)


configure_telemetry(api)


class PodSummary(BaseModel):
    """Input model describing a pod snapshot used by the scorer."""

    id: str
    name: str
    namespace: str
    status: str
    cpu: str
    memory: str
    age: str
    restarts: int


class CPUPoint(BaseModel):
    """Single CPU usage sample used to compute short trend signals."""

    time: str
    value: int


class NodeSummary(BaseModel):
    """Input model describing a node snapshot used by the scorer."""

    name: str
    status: str
    roles: str
    age: str
    version: str
    cpuUsage: str
    memUsage: str
    cpuHistory: list[CPUPoint] = Field(default_factory=list)


class K8sEvent(BaseModel):
    """Cluster event model used for warning correlation and weighting."""

    type: str = ""
    reason: str = ""
    age: str = ""
    from_: str = Field(default="", alias="from")
    message: str = ""
    count: int | None = None


class PredictionSignal(BaseModel):
    """Structured explanation signal attached to a prediction result."""

    key: str
    value: str


class IncidentPrediction(BaseModel):
    """Predictive incident item returned to the KubeLens backend."""

    id: str
    resourceKind: str
    resource: str
    namespace: str | None = None
    riskScore: int
    confidence: int
    summary: str
    recommendation: str
    signals: list[PredictionSignal] = Field(default_factory=list)


class PredictionRequest(BaseModel):
    """Prediction request payload accepted by ``POST /predict``."""

    pods: list[PodSummary] = Field(default_factory=list)
    nodes: list[NodeSummary] = Field(default_factory=list)
    events: list[K8sEvent] = Field(default_factory=list)
    timestamp: str | None = None


class PredictionResponse(BaseModel):
    """Prediction response payload returned by ``POST /predict``."""

    source: str
    generatedAt: str
    items: list[IncidentPrediction]


@api.get("/healthz")
def healthz() -> dict:
    """Return predictor liveness status.

    Returns:
        dict: A static health payload with ``{"status": "ok"}``.
    """

    return {"status": "ok"}


def require_predictor_secret(
    x_predictor_secret: str | None = Header(default=None, alias="X-Predictor-Secret"),
) -> None:
    """Validate optional shared-secret authentication for predictor requests.

    If ``PREDICTOR_SHARED_SECRET`` is not configured, all requests are allowed.
    Otherwise callers must provide ``X-Predictor-Secret`` with the exact value.

    Args:
        x_predictor_secret: Header value supplied by the caller.

    Raises:
        HTTPException: If the shared secret is configured but does not match.
    """

    expected = os.getenv("PREDICTOR_SHARED_SECRET", "").strip()
    if expected == "":
        return
    if (x_predictor_secret or "").strip() != expected:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="unauthorized predictor request")


def get_optional_ml_model() -> object | None:
    """Load the optional joblib model configured for pod risk blending.

    The predictor remains deterministic by default. Operators can opt in to ML
    scoring by setting ``PREDICTOR_MODEL_PATH`` and installing the optional ML
    requirements. Model loading is cached per path to avoid repeated disk I/O.

    Returns:
        object | None: Loaded model object, or ``None`` when ML is disabled or unavailable.
    """

    path = os.getenv("PREDICTOR_MODEL_PATH", "").strip()
    if path == "":
        return None

    global _ml_model_cache_path, _ml_model_cache
    if _ml_model_cache_path == path:
        return _ml_model_cache

    try:
        import joblib

        _ml_model_cache = joblib.load(path)
        _ml_model_cache_path = path
        logger.info("Loaded optional predictor ML model from %s", path)
        return _ml_model_cache
    except Exception as exc:  # pragma: no cover - environment dependent startup path
        _ml_model_cache = None
        _ml_model_cache_path = path
        logger.warning("Optional predictor ML model disabled: %s", exc)
        return None


def pod_ml_features(pod: PodSummary, cpu_milli: int, mem_mi: int) -> list[float]:
    """Build the stable pod feature vector consumed by optional ML models."""

    return [float(pod.restarts), float(max(cpu_milli, 0)), float(max(mem_mi, 0))]


def score_pod_with_ml(pod: PodSummary, cpu_milli: int, mem_mi: int) -> int | None:
    """Return an optional ML risk score for a pod.

    Models may expose either ``predict_proba`` with a positive-class column or
    ``predict`` returning a value in ``[0, 1]`` or ``[0, 100]``.
    """

    model = get_optional_ml_model()
    if model is None:
        return None

    features = [pod_ml_features(pod, cpu_milli, mem_mi)]
    try:
        if hasattr(model, "predict_proba"):
            probabilities = model.predict_proba(features)
            first_row = probabilities[0]
            probability = float(first_row[1] if len(first_row) > 1 else first_row[0])
        elif hasattr(model, "predict"):
            prediction = model.predict(features)
            probability = float(prediction[0])
        else:
            logger.warning("Optional predictor ML model has no predict interface")
            return None
    except Exception as exc:  # pragma: no cover - depends on external model behavior
        logger.warning("Optional predictor ML model inference failed: %s", exc)
        return None

    if probability > 1:
        probability = probability / 100
    probability = max(0.0, min(1.0, probability))
    return clamp(round(probability * 100), 0, 100)


def blend_risk_score(deterministic_score: int, ml_score: int) -> int:
    """Blend deterministic risk with optional ML risk while preserving explainability."""

    blended = round(deterministic_score * 0.60 + ml_score * 0.40)
    return clamp(max(deterministic_score, blended), 0, 100)


@api.post("/predict", response_model=PredictionResponse)
# Intentionally sync: prediction is CPU-bound in-memory scoring with no blocking I/O.
def predict(request: PredictionRequest, _: None = Depends(require_predictor_secret)) -> PredictionResponse:
    """Score pod and node risk from a cluster snapshot.

    The scorer combines status, restart counts, resource usage, warning-event
    correlation, and simple node CPU trend heuristics. Results are sorted by
    risk and confidence and capped to the top 10 items.

    Args:
        request: Cluster snapshot containing pods, nodes, and events.
        _: Dependency placeholder enforcing optional shared-secret checks.

    Returns:
        PredictionResponse: Ranked prediction items with evidence signals.
    """

    items: list[IncidentPrediction] = []

    for pod in request.pods:
        score = 0
        signals: list[PredictionSignal] = []
        status = pod.status.lower().strip()
        resource_warnings = count_resource_warning_events(request.events, pod.name, pod.namespace)
        cpu_milli, cpu_known = parse_cpu_milli(pod.cpu)
        mem_mi, mem_known = parse_memory_mi(pod.memory)

        if status == "failed":
            score += 62
            signals.append(PredictionSignal(key="status", value="Failed"))
        elif status == "pending":
            score += 34
            signals.append(PredictionSignal(key="status", value="Pending"))
        elif status == "unknown":
            score += 20
            signals.append(PredictionSignal(key="status", value="Unknown"))

        if pod.restarts > 0:
            score += min(42, pod.restarts * 8)
            signals.append(PredictionSignal(key="restarts", value=str(pod.restarts)))

        if cpu_milli >= 400:
            score += 10
            signals.append(PredictionSignal(key="cpu", value=pod.cpu))

        if mem_mi >= 512:
            score += 10
            signals.append(PredictionSignal(key="memory", value=pod.memory))

        if resource_warnings > 0 and status != "running":
            score += min(12, resource_warnings * 2)

        ml_score = score_pod_with_ml(pod, cpu_milli, mem_mi)
        if ml_score is not None:
            score = blend_risk_score(score, ml_score)
            signals.append(PredictionSignal(key="mlRisk", value=f"{ml_score}%"))

        score = clamp(score, 0, 100)
        if score < 35:
            continue

        recommendation = "Inspect pod events and logs; verify dependencies and resource limits."
        if status == "pending":
            recommendation = "Validate scheduling constraints, image pulls, and resource requests."
        elif status == "failed":
            recommendation = "Check crash loops, roll back unstable changes, and validate readiness probes."

        confidence = confidence_from_evidence(
            strong_status=status in {"failed", "pending"},
            signal_count=len(signals),
            metric_known=int(cpu_known) + int(mem_known),
            metric_signal_count=int(cpu_milli >= 400) + int(mem_mi >= 512),
            warning_matches=resource_warnings,
            restart_signal=pod.restarts > 0,
        )
        items.append(
            IncidentPrediction(
                id=f"pod-{pod.id}",
                resourceKind="Pod",
                resource=pod.name,
                namespace=pod.namespace,
                riskScore=score,
                confidence=confidence,
                summary=f"{pod.name} shows elevated risk with status {pod.status} and {pod.restarts} restarts.",
                recommendation=recommendation,
                signals=signals,
            )
        )

    for node in request.nodes:
        score = 0
        signals: list[PredictionSignal] = []
        cpu_pct, cpu_known = parse_percent(node.cpuUsage)
        mem_pct, mem_known = parse_percent(node.memUsage)
        resource_warnings = count_resource_warning_events(request.events, node.name, None)
        cpu_trend = compute_trend(node.cpuHistory)

        if node.status.strip().lower() == "notready":
            score += 75
            signals.append(PredictionSignal(key="status", value="NotReady"))

        if cpu_known and cpu_pct >= 90:
            score += 20
            signals.append(PredictionSignal(key="cpuUsage", value=node.cpuUsage))

        if mem_known and mem_pct >= 90:
            score += 20
            signals.append(PredictionSignal(key="memUsage", value=node.memUsage))

        if cpu_trend >= 20 and cpu_known and cpu_pct >= 80:
            score += 10
            signals.append(PredictionSignal(key="cpuTrend", value=f"+{cpu_trend}%"))

        if resource_warnings > 0 and node.status.strip().lower() != "ready":
            score += min(10, resource_warnings * 2)

        score = clamp(score, 0, 100)
        if score < 45:
            continue

        confidence = confidence_from_evidence(
            strong_status=node.status.strip().lower() == "notready",
            signal_count=len(signals),
            metric_known=int(cpu_known) + int(mem_known),
            metric_signal_count=int(cpu_known and cpu_pct >= 90) + int(mem_known and mem_pct >= 90),
            warning_matches=resource_warnings,
            restart_signal=False,
        )
        recommendation = "Inspect kubelet health, pressure conditions, and workload distribution."
        if cpu_trend >= 20 and cpu_known and cpu_pct >= 80:
            recommendation = "CPU usage is trending up quickly; review noisy neighbors and consider scaling."

        items.append(
            IncidentPrediction(
                id=f"node-{node.name.lower()}",
                resourceKind="Node",
                resource=node.name,
                riskScore=score,
                confidence=confidence,
                summary=f"Node {node.name} shows elevated operational risk.",
                recommendation=recommendation,
                signals=signals,
            )
        )

    items.sort(key=lambda x: (x.riskScore, x.confidence), reverse=True)
    items = items[:10]

    return PredictionResponse(
        source="python-fastapi",
        generatedAt=datetime.now(timezone.utc).isoformat(),
        items=items,
    )


def count_resource_warning_events(events: list[K8sEvent], resource: str, namespace: str | None) -> int:
    """Count warning-like events that reference a resource.

    Args:
        events: Cluster events from the request payload.
        resource: Resource name to match in event reason/message/source fields.
        namespace: Optional namespace hint used for matching.

    Returns:
        int: Weighted count of matching warning events.
    """

    resource_name = resource.strip().lower()
    namespace_name = (namespace or "").strip().lower()
    total = 0

    for event in events:
        event_type = event.type.strip().lower()
        event_reason = event.reason.strip().lower()
        if event_type != "warning" and event_reason not in {"backoff", "failed", "unhealthy", "oomkilled"}:
            continue

        haystack = f"{event.reason} {event.message} {event.from_}".lower()
        if resource_name not in haystack and (namespace_name == "" or namespace_name not in haystack):
            continue

        total += max(1, event.count or 1)

    return total


def compute_trend(history: list[CPUPoint]) -> int:
    """Compute non-negative delta between first and last CPU history points.

    Args:
        history: Ordered CPU history points.

    Returns:
        int: Positive increase between the first and last sample, else 0.
    """

    if len(history) < 2:
        return 0
    start = history[0].value
    end = history[-1].value
    return max(0, end - start)


def confidence_from_evidence(
    *,
    strong_status: bool,
    signal_count: int,
    metric_known: int,
    metric_signal_count: int,
    warning_matches: int,
    restart_signal: bool,
) -> int:
    """Estimate prediction confidence from available evidence signals.

    Args:
        strong_status: Whether a high-confidence status signal exists.
        signal_count: Number of emitted signals for the candidate.
        metric_known: Number of known metric dimensions.
        metric_signal_count: Number of metrics crossing signal thresholds.
        warning_matches: Count of correlated warning events.
        restart_signal: Whether restart behavior contributes evidence.

    Returns:
        int: Confidence score clamped to [35, 96].
    """

    confidence = 35
    if strong_status:
        confidence += 18

    confidence += min(24, signal_count * 6)
    confidence += min(16, metric_known * 8)
    confidence += min(10, metric_signal_count * 5)
    confidence += min(12, warning_matches * 3)
    if restart_signal:
        confidence += 6

    if signal_count <= 1:
        confidence -= 8
    if metric_known == 0 and not strong_status:
        confidence -= 10

    bounded_confidence = clamp(confidence, 35, 96)
    # Confidence is deliberately clamped to a stable range for UI comparability.
    assert 35 <= bounded_confidence <= 96
    return bounded_confidence


def parse_cpu_milli(value: str) -> tuple[int, bool]:
    """Parse CPU usage to milli-CPU units.

    Args:
        value: Raw CPU value such as ``450m`` or ``0.5``.

    Returns:
        tuple[int, bool]: Parsed milli-CPU value and whether parsing succeeded.
    """

    raw = value.strip().lower()
    if not raw or raw == "n/a":
        return 0, False
    try:
        if raw.endswith("m"):
            return int(float(raw[:-1] or 0)), True
        return int(float(raw) * 1000), True
    except ValueError:
        return 0, False


def parse_memory_mi(value: str) -> tuple[int, bool]:
    """Parse memory usage to mebibytes.

    Supported suffixes are ``Ki``, ``Mi``, ``Gi``, and ``B``.

    Args:
        value: Raw memory value.

    Returns:
        tuple[int, bool]: Parsed Mi value and whether parsing succeeded.
    """

    raw = value.strip().lower()
    if not raw or raw == "n/a":
        return 0, False
    try:
        if raw.endswith("mi"):
            return int(float(raw[:-2] or 0)), True
        if raw.endswith("gi"):
            return int(float(raw[:-2] or 0) * 1024), True
        if raw.endswith("ki"):
            return int(float(raw[:-2] or 0) / 1024), True
        if raw.endswith("b"):
            return int(float(raw[:-1] or 0) / (1024 * 1024)), True
    except ValueError:
        return 0, False
    # Reached when input is numeric but has an unsupported suffix (for example "500mb").
    return 0, False


def parse_percent(value: str) -> tuple[float, bool]:
    """Parse percentage values and clamp to ``[0.0, 100.0]``.

    Args:
        value: Raw percent value, optionally including ``%``.

    Returns:
        tuple[float, bool]: Parsed percentage and success flag.
    """

    raw = value.strip().lower().replace("%", "")
    if not raw or raw == "n/a":
        return 0.0, False
    try:
        parsed = float(raw)
    except ValueError:
        return 0.0, False
    return max(0.0, min(100.0, parsed)), True


def clamp(value: int, low: int, high: int) -> int:
    """Clamp an integer to an inclusive range.

    Args:
        value: Input integer.
        low: Inclusive lower bound.
        high: Inclusive upper bound.

    Returns:
        int: Clamped value.
    """

    if value < low:
        return low
    if value > high:
        return high
    return value
