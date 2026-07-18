"""Predictor API and scoring unit tests."""

import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest
from fastapi import HTTPException
from predictor.app import main as predictor_main
from predictor.app.main import (
    DEFAULT_ML_FEATURES,
    IncidentPrediction,
    K8sEvent,
    ModelMetadata,
    PredictionRequest,
    PredictionResponse,
    blend_risk_score,
    build_pod_ml_feature_set,
    confidence_from_evidence,
    count_resource_warning_events,
    healthz,
    metadata_is_stale,
    ml_feature_set_completeness,
    model_health_endpoint,
    parse_cpu_milli,
    parse_memory_mi,
    predict,
    require_predictor_secret,
    telemetry_endpoint,
)
from pydantic import ValidationError


@pytest.fixture(autouse=True)
def clear_predictor_ml_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Keep optional ML environment from leaking across tests."""

    for key in (
        "PREDICTOR_MODE",
        "PREDICTOR_MODEL_PATH",
        "PREDICTOR_MODEL_METADATA_PATH",
        "PREDICTOR_MIN_FEATURE_COMPLETENESS",
        "PREDICTOR_MAX_MODEL_AGE_HOURS",
    ):
        monkeypatch.delenv(key, raising=False)
    predictor_main._ml_model_cache_path = None
    predictor_main._ml_model_cache = None
    predictor_main._ml_metadata_cache_path = None
    predictor_main._ml_metadata_cache = None
    with predictor_main._telemetry_lock:
        predictor_main._telemetry_state.update(
            {
                "totalRequests": 0,
                "generatedPredictions": 0,
                "mlInferenceAttempts": 0,
                "mlInferenceFailures": 0,
                "mlDisagreements": 0,
                "featureMissingRate": 0.0,
                "averageLatencyMs": 0.0,
                "lastLatencyMs": 0.0,
                "scoreBuckets": {"low": 0, "medium": 0, "high": 0, "critical": 0},
                "lastPredictionAt": "",
            }
        )


def predict_payload(payload: dict) -> PredictionResponse:
    """Validate a raw payload and run prediction without Starlette TestClient."""

    return predict(PredictionRequest.model_validate(payload))


def signal_payloads(item: IncidentPrediction) -> list[dict[str, str]]:
    """Return prediction signals in the same shape the JSON API emits."""

    return [signal.model_dump() for signal in item.signals]


def write_model_metadata(tmp_path: Path, training_timestamp: str | None = None) -> Path:
    """Write a valid model metadata sidecar for ML governance tests."""

    metadata_path = tmp_path / "model.json"
    metadata_path.write_text(
        json.dumps(
            {
                "modelVersion": "pod-risk-2026-07",
                "gitCommit": "abc1234",
                "trainingDataWindow": "2026-06-01/2026-07-01",
                "featureList": ["restarts", "cpu_milli", "memory_mi"],
                "labelDefinition": "incident within 30 minutes",
                "evaluationMetrics": {"recall": 0.84, "precision": 0.73},
                "promotionGates": {"recall": 0.80, "precision": 0.70},
                "calibratedThreshold": 0.72,
                "calibrationMethod": "f1_threshold_tuning",
                "trainingTimestamp": training_timestamp or datetime.now(timezone.utc).isoformat(),
                "ownerReviewer": "sre-platform",
            }
        ),
        encoding="utf-8",
    )
    return metadata_path


def test_healthz_ok() -> None:
    """The health endpoint returns a static success payload."""

    assert healthz() == {"status": "ok"}


def test_model_health_reports_deterministic_default(monkeypatch: pytest.MonkeyPatch) -> None:
    """The predictor does not load or blend ML unless the mode opts in."""

    monkeypatch.setenv("PREDICTOR_MODEL_PATH", "/tmp/does-not-exist.joblib")

    data = model_health_endpoint()
    assert data.mode == "deterministic"
    assert data.enabled is False
    assert data.usableForBlending is False
    assert data.modelLoaded is False
    assert data.requiredFeatures == DEFAULT_ML_FEATURES
    assert data.lastError == ""


def test_model_health_reports_shadow_metadata(monkeypatch: pytest.MonkeyPatch, tmp_path) -> None:
    """Shadow mode exposes model metadata without marking it blendable."""

    metadata_path = write_model_metadata(tmp_path)
    monkeypatch.setenv("PREDICTOR_MODE", "shadow")
    monkeypatch.setenv("PREDICTOR_MODEL_METADATA_PATH", str(metadata_path))
    monkeypatch.setattr(predictor_main, "get_optional_ml_model", lambda: object())

    data = model_health_endpoint()
    assert data.mode == "shadow"
    assert data.enabled is True
    assert data.usableForBlending is False
    assert data.modelLoaded is True
    assert data.metadataLoaded is True
    assert data.modelVersion == "pod-risk-2026-07"
    assert data.stale is False
    assert data.calibratedThreshold == 0.72
    assert data.calibrationMethod == "f1_threshold_tuning"
    assert data.evaluationMetrics["recall"] == 0.84
    assert data.promotionGates["precision"] == 0.70


def test_predict_returns_risk_items() -> None:
    """A failing pod with warning signals produces at least one prediction."""

    payload = {
        "pods": [
            {
                "id": "p1",
                "name": "payment-gateway",
                "namespace": "prod",
                "status": "Failed",
                "cpu": "450m",
                "memory": "600Mi",
                "age": "5m",
                "restarts": 4,
            }
        ],
        "nodes": [],
        "events": [{"type": "Warning", "reason": "BackOff", "age": "1m", "from": "kubelet", "message": "loop"}],
    }

    data = predict_payload(payload)
    assert data.source == "python-fastapi"
    assert len(data.items) == 1
    assert data.items[0].riskScore >= 35
    telemetry = telemetry_endpoint()
    assert telemetry.totalRequests == 1
    assert telemetry.generatedPredictions == 1
    assert telemetry.lastLatencyMs >= 0
    assert telemetry.averageLatencyMs >= 0
    assert telemetry.lastPredictionAt != ""


def test_predict_handles_invalid_usage_values() -> None:
    """Invalid usage strings are tolerated and still return a valid response."""

    payload = {
        "pods": [
            {
                "id": "p2",
                "name": "auth",
                "namespace": "prod",
                "status": "Pending",
                "cpu": "not-a-number",
                "memory": "broken",
                "age": "1m",
                "restarts": 2,
            }
        ],
        "nodes": [
            {
                "name": "node-1",
                "status": "NotReady",
                "roles": "worker",
                "age": "1d",
                "version": "1.31",
                "cpuUsage": "bad%",
                "memUsage": "also-bad",
            }
        ],
    }

    assert predict_payload(payload).source == "python-fastapi"


def test_predict_scores_not_ready_node_with_hot_metrics() -> None:
    """NotReady nodes with saturated metrics are scored as elevated risk."""

    payload = {
        "pods": [],
        "nodes": [
            {
                "name": "node-hot-1",
                "status": "NotReady",
                "roles": "worker",
                "age": "3d",
                "version": "1.31",
                "cpuUsage": "95%",
                "memUsage": "92%",
                "cpuHistory": [
                    {"time": "10:00", "value": 72},
                    {"time": "10:05", "value": 96},
                ],
            }
        ],
        "events": [{"type": "Warning", "reason": "Failed", "age": "1m", "from": "kubelet", "message": "node-hot-1"}],
    }

    data = predict_payload(payload)
    node_item = next((item for item in data.items if item.resourceKind == "Node"), None)
    assert node_item is not None
    assert node_item.riskScore >= 45
    assert node_item.confidence >= 70


def test_predict_blends_optional_ml_score(monkeypatch: pytest.MonkeyPatch, tmp_path) -> None:
    """Optional ML scoring honors the metadata-declared legacy feature order."""

    class HighRiskModel:
        def predict_proba(self, features: list[list[float]]) -> list[list[float]]:
            assert features == [[0.0, 50.0, 128.0]]
            return [[0.05, 0.95]]

    monkeypatch.setenv("PREDICTOR_MODE", "blended")
    monkeypatch.setenv("PREDICTOR_MODEL_METADATA_PATH", str(write_model_metadata(tmp_path)))
    monkeypatch.setattr(predictor_main, "get_optional_ml_model", lambda: HighRiskModel())
    payload = {
        "pods": [
            {
                "id": "p-ml",
                "name": "checkout",
                "namespace": "prod",
                "status": "Running",
                "cpu": "50m",
                "memory": "128Mi",
                "age": "3m",
                "restarts": 0,
            }
        ],
        "nodes": [],
        "events": [],
    }

    item = predict_payload(payload).items[0]
    assert item.resource == "checkout"
    assert item.riskScore == 38
    assert {"key": "mlFeatureCompleteness", "value": "100%"} in signal_payloads(item)
    assert {"key": "mlRisk", "value": "95%"} in signal_payloads(item)
    assert {"key": "mlThreshold", "value": "0.72"} in signal_payloads(item)


def test_predict_shadow_mode_emits_ml_without_blending(monkeypatch: pytest.MonkeyPatch) -> None:
    """Shadow ML can use the richer default feature contract without changing risk."""

    class HighRiskModel:
        def predict_proba(self, features: list[list[float]]) -> list[list[float]]:
            assert features == [
                [
                    1.0,
                    50.0,
                    128.0,
                    0.0,
                    1.0,
                    0.0,
                    3.0,
                    0.0,
                    0.0,
                    1.0,
                    0.0,
                    20.0,
                    0.0,
                    0.0,
                    3.0,
                    0.0,
                    0.0,
                ]
            ]
            return [[0.05, 0.95]]

    monkeypatch.setenv("PREDICTOR_MODE", "shadow")
    monkeypatch.setattr(predictor_main, "get_optional_ml_model", lambda: HighRiskModel())
    payload = {
        "pods": [
            {
                "id": "p-shadow",
                "name": "checkout",
                "namespace": "prod",
                "status": "Pending",
                "cpu": "50m",
                "memory": "128Mi",
                "age": "3m",
                "restarts": 1,
            }
        ],
        "nodes": [],
        "events": [],
    }

    item = predict_payload(payload).items[0]
    assert item.riskScore == 42
    assert {"key": "mlDisagreement", "value": "53%"} in signal_payloads(item)
    assert {"key": "mlShadowRisk", "value": "95%"} in signal_payloads(item)
    assert {"key": "mlMode", "value": "shadow"} in signal_payloads(item)
    assert {"key": "mlRisk", "value": "95%"} not in signal_payloads(item)
    telemetry = telemetry_endpoint()
    assert telemetry.mlInferenceAttempts == 1
    assert telemetry.mlDisagreements == 1


def test_pod_ml_features_include_snapshot_context() -> None:
    """The default ML contract derives richer pod, namespace, node, and event features."""

    request = PredictionRequest.model_validate(
        {
            "pods": [
                {
                    "id": "p-rich",
                    "name": "checkout",
                    "namespace": "prod",
                    "nodeName": "node-hot",
                    "status": "Pending",
                    "cpu": "600m",
                    "memory": "1Gi",
                    "age": "30m",
                    "restarts": 3,
                    "phaseDuration": "10m",
                    "previousIncidents": 2,
                    "cpuHistory": [{"time": "10:00", "value": 40}, {"time": "10:05", "value": 65}],
                    "memoryHistory": [{"time": "10:00", "value": 70}, {"time": "10:05", "value": 88}],
                },
                {
                    "id": "p-ok",
                    "name": "worker",
                    "namespace": "prod",
                    "status": "Running",
                    "cpu": "100m",
                    "memory": "128Mi",
                    "age": "1h",
                    "restarts": 0,
                },
            ],
            "nodes": [
                {
                    "name": "node-hot",
                    "status": "NotReady",
                    "roles": "worker",
                    "age": "2d",
                    "version": "1.31",
                    "cpuUsage": "91%",
                    "memUsage": "88%",
                }
            ],
            "events": [
                {
                    "type": "Warning",
                    "reason": "ErrImagePull",
                    "namespace": "prod",
                    "resource": "checkout",
                    "message": "checkout image pull failed",
                    "count": 2,
                },
                {
                    "type": "Warning",
                    "reason": "BackOff",
                    "namespace": "prod",
                    "message": "checkout backing off",
                },
            ],
        }
    )
    pod = request.pods[0]
    cpu_milli, cpu_known = parse_cpu_milli(pod.cpu)
    mem_mi, mem_known = parse_memory_mi(pod.memory)
    feature_set = build_pod_ml_feature_set(
        pod,
        request,
        cpu_milli=cpu_milli,
        cpu_known=cpu_known,
        mem_mi=mem_mi,
        mem_known=mem_known,
        resource_warnings=count_resource_warning_events(request.events, pod.name, pod.namespace),
    )

    assert feature_set.values["status_pending"] == 1.0
    assert feature_set.values["pod_age_minutes"] == 30.0
    assert feature_set.values["warning_events"] == 3.0
    assert feature_set.values["namespace_warning_events"] == 3.0
    assert feature_set.values["namespace_non_running_ratio"] == 0.5
    assert feature_set.values["node_not_ready"] == 1.0
    assert feature_set.values["restart_velocity_per_hour"] == 6.0
    assert feature_set.values["cpu_trend_delta"] == 25.0
    assert feature_set.values["memory_trend_delta"] == 18.0
    assert feature_set.values["phase_duration_minutes"] == 10.0
    assert feature_set.values["image_pull_backoff_events"] == 2.0
    assert feature_set.values["previous_incidents"] == 2.0
    assert ml_feature_set_completeness(feature_set, DEFAULT_ML_FEATURES) > 0.90


def test_blended_ml_requires_feature_completeness(monkeypatch: pytest.MonkeyPatch, tmp_path) -> None:
    """Blended ML is blocked when required runtime features are missing."""

    class HighRiskModel:
        def predict_proba(self, features: list[list[float]]) -> list[list[float]]:
            assert features == [[2.0, 0.0, 0.0]]
            return [[0.05, 0.95]]

    monkeypatch.setenv("PREDICTOR_MODE", "blended")
    monkeypatch.setenv("PREDICTOR_MODEL_METADATA_PATH", str(write_model_metadata(tmp_path)))
    monkeypatch.setenv("PREDICTOR_MIN_FEATURE_COMPLETENESS", "1.0")
    monkeypatch.setattr(predictor_main, "get_optional_ml_model", lambda: HighRiskModel())
    payload = {
        "pods": [
            {
                "id": "p-incomplete",
                "name": "checkout",
                "namespace": "prod",
                "status": "Pending",
                "cpu": "bad",
                "memory": "bad",
                "age": "3m",
                "restarts": 2,
            }
        ],
        "nodes": [],
        "events": [],
    }

    item = predict_payload(payload).items[0]
    assert item.riskScore == 50
    assert {"key": "mlRiskBlocked", "value": "feature-completeness 33%"} in signal_payloads(item)
    assert {"key": "mlShadowRisk", "value": "95%"} in signal_payloads(item)
    assert {"key": "mlRisk", "value": "95%"} not in signal_payloads(item)


def test_blended_ml_requires_model_metadata(monkeypatch: pytest.MonkeyPatch) -> None:
    """Blended ML does not affect risk when model metadata is unavailable."""

    class HighRiskModel:
        def predict_proba(self, features: list[list[float]]) -> list[list[float]]:
            raise AssertionError("inference should be blocked before model execution")

    monkeypatch.setenv("PREDICTOR_MODE", "blended")
    monkeypatch.setattr(predictor_main, "get_optional_ml_model", lambda: HighRiskModel())
    payload = {
        "pods": [
            {
                "id": "p-unversioned",
                "name": "checkout",
                "namespace": "prod",
                "status": "Pending",
                "cpu": "50m",
                "memory": "128Mi",
                "age": "3m",
                "restarts": 2,
            }
        ],
        "nodes": [],
        "events": [],
    }

    item = predict_payload(payload).items[0]
    assert item.riskScore == 50
    assert {"key": "mlRiskBlocked", "value": "metadata-unavailable"} in signal_payloads(item)
    assert {"key": "mlRisk", "value": "95%"} not in signal_payloads(item)


def test_predict_rejects_invalid_contract() -> None:
    """Malformed payloads are rejected by Pydantic validation."""

    with pytest.raises(ValidationError):
        PredictionRequest.model_validate({"pods": "bad"})


def test_predict_requires_shared_secret_when_configured(monkeypatch: pytest.MonkeyPatch) -> None:
    """Secret-protected mode requires the matching predictor header."""

    monkeypatch.setenv("PREDICTOR_SHARED_SECRET", "secret-123")
    payload = {"pods": [], "nodes": [], "events": []}

    with pytest.raises(HTTPException) as exc_info:
        require_predictor_secret(x_predictor_secret=None)
    assert exc_info.value.status_code == 401

    require_predictor_secret(x_predictor_secret="secret-123")
    assert predict_payload(payload).source == "python-fastapi"


def test_confidence_from_evidence_rewards_richer_signals() -> None:
    """Confidence increases when evidence quantity and strength improve."""

    sparse = confidence_from_evidence(
        strong_status=False,
        signal_count=1,
        metric_known=0,
        metric_signal_count=0,
        warning_matches=0,
        restart_signal=False,
    )
    rich = confidence_from_evidence(
        strong_status=True,
        signal_count=4,
        metric_known=2,
        metric_signal_count=2,
        warning_matches=3,
        restart_signal=True,
    )

    assert rich > sparse


def test_blend_risk_score_never_lowers_deterministic_score() -> None:
    """ML blending cannot reduce the deterministic risk score."""

    assert blend_risk_score(80, 10) == 80
    assert blend_risk_score(10, 90) == 42


def test_metadata_is_stale_after_configured_age(monkeypatch: pytest.MonkeyPatch) -> None:
    """Model metadata freshness is enforced by the configured max age."""

    monkeypatch.setenv("PREDICTOR_MAX_MODEL_AGE_HOURS", "1")
    metadata = ModelMetadata(
        modelVersion="old-model",
        trainingTimestamp=(datetime.now(timezone.utc) - timedelta(hours=2)).isoformat(),
    )

    assert metadata_is_stale(metadata) is True


def test_count_resource_warning_events_matches_message_and_count() -> None:
    """Warning correlation counts event matches and honors event count fields."""

    events = [
        K8sEvent(
            type="Warning",
            reason="BackOff",
            age="1m",
            **{"from": "kubelet"},
            message="pod payment-gateway in namespace production restarted repeatedly",
            count=3,
        ),
        K8sEvent(
            type="Warning",
            reason="Failed",
            age="2m",
            **{"from": "kubelet"},
            message="node node-worker-3 kubelet not ready",
            count=2,
        ),
    ]

    assert count_resource_warning_events(events, "payment-gateway", "production") == 3
    assert count_resource_warning_events(events, "node-worker-3", None) == 2


def test_parse_memory_mi_supports_gi_suffix() -> None:
    """Gi memory suffix is converted to Mi units."""

    value, known = parse_memory_mi("1Gi")
    assert known is True
    assert value == 1024


def test_parse_cpu_milli_supports_whole_cpu_units() -> None:
    """Whole CPU units are converted to milli-CPU."""

    value, known = parse_cpu_milli("2")
    assert known is True
    assert value == 2000
