"""Predictor API and scoring unit tests."""

from fastapi.testclient import TestClient
from predictor.app import main as predictor_main
from predictor.app.main import (
    K8sEvent,
    api,
    blend_risk_score,
    confidence_from_evidence,
    count_resource_warning_events,
    parse_cpu_milli,
    parse_memory_mi,
)

client = TestClient(api)


def test_healthz_ok() -> None:
    """The health endpoint returns a static success payload."""

    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


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

    response = client.post("/predict", json=payload)
    assert response.status_code == 200

    data = response.json()
    assert data["source"] == "python-fastapi"
    assert len(data["items"]) == 1
    assert data["items"][0]["riskScore"] >= 35


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

    response = client.post("/predict", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert data["source"] == "python-fastapi"


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

    response = client.post("/predict", json=payload)
    assert response.status_code == 200
    data = response.json()
    node_item = next((item for item in data["items"] if item["resourceKind"] == "Node"), None)
    assert node_item is not None
    assert node_item["riskScore"] >= 45
    assert node_item["confidence"] >= 70


def test_predict_blends_optional_ml_score(monkeypatch) -> None:
    """Optional ML scoring can surface a high-risk running pod."""

    class HighRiskModel:
        def predict_proba(self, features: list[list[float]]) -> list[list[float]]:
            assert features == [[0.0, 50.0, 128.0]]
            return [[0.05, 0.95]]

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

    response = client.post("/predict", json=payload)
    assert response.status_code == 200
    item = response.json()["items"][0]
    assert item["resource"] == "checkout"
    assert item["riskScore"] == 38
    assert {"key": "mlRisk", "value": "95%"} in item["signals"]


def test_predict_rejects_invalid_contract() -> None:
    """Malformed payloads are rejected by FastAPI validation."""

    response = client.post("/predict", json={"pods": "bad"})
    assert response.status_code == 422


def test_predict_requires_shared_secret_when_configured(monkeypatch) -> None:
    """Secret-protected mode requires the matching predictor header."""

    monkeypatch.setenv("PREDICTOR_SHARED_SECRET", "secret-123")
    payload = {"pods": [], "nodes": [], "events": []}

    unauthorized = client.post("/predict", json=payload)
    assert unauthorized.status_code == 401

    authorized = client.post("/predict", json=payload, headers={"X-Predictor-Secret": "secret-123"})
    assert authorized.status_code == 200
    monkeypatch.delenv("PREDICTOR_SHARED_SECRET", raising=False)


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
