"""Optional predictor ML trainer tests."""

import json

import pytest

pytest.importorskip("joblib")
pd = pytest.importorskip("pandas")
pytest.importorskip("sklearn")

from predictor.app.ml_prototype import (  # noqa: E402
    FEATURE_COLUMNS,
    best_f1_threshold,
    build_feature_frame,
    promotion_gate_thresholds,
    train_simple_model,
    validate_evaluation_gates,
)


def test_train_simple_model_writes_metadata_sidecar(tmp_path) -> None:
    """Training writes both the model artifact and runtime metadata."""

    data_path = tmp_path / "training.csv"
    data_path.write_text(
        "\n".join(
            [
                "timestamp,restarts,cpu,memory,label",
                "2026-07-01T00:00:00Z,0,50m,128Mi,0",
                "2026-07-01T00:05:00Z,1,90m,192Mi,0",
                "2026-07-01T00:10:00Z,2,180m,256Mi,0",
                "2026-07-01T00:15:00Z,1,120m,180Mi,0",
                "2026-07-01T00:20:00Z,5,900m,1Gi,1",
                "2026-07-01T00:25:00Z,6,850m,900Mi,1",
                "2026-07-01T00:30:00Z,4,700m,768Mi,1",
                "2026-07-01T00:35:00Z,7,950m,1Gi,1",
            ]
        ),
        encoding="utf-8",
    )
    model_path = tmp_path / "pod-risk.joblib"
    metadata_path = tmp_path / "pod-risk.metadata.json"

    saved_path = train_simple_model(
        str(data_path),
        str(model_path),
        metadata_save_path=str(metadata_path),
        model_version="pod-risk-test",
        git_commit="abc1234",
        owner_reviewer="sre-platform",
    )

    assert saved_path == model_path
    assert model_path.exists()
    payload = json.loads(metadata_path.read_text(encoding="utf-8"))
    assert payload["modelVersion"] == "pod-risk-test"
    assert payload["gitCommit"] == "abc1234"
    assert payload["featureList"] == list(FEATURE_COLUMNS)
    assert payload["labelDefinition"] == "label=incident within rollout horizon"
    assert payload["ownerReviewer"] == "sre-platform"
    assert payload["calibratedThreshold"] == 0.5
    assert payload["calibrationMethod"] == "manual_threshold"
    assert payload["promotionGates"] == {"precision": 0.0, "recall": 0.0, "rocAuc": 0.0}
    assert "2026-07-01T00:00:00+00:00/2026-07-01T00:35:00+00:00" == payload["trainingDataWindow"]
    assert "precision" in payload["evaluationMetrics"]
    assert "recall" in payload["evaluationMetrics"]


def test_build_feature_frame_normalizes_extended_features(tmp_path) -> None:
    """Training data can provide the richer runtime feature contract."""

    data_path = tmp_path / "training.csv"
    data_path.write_text(
        "\n".join(
            [
                "restarts,cpu,memory,status,age,phase_duration,node_status,warning_events,namespace_warning_events,namespace_pressure,cpu_trend,memory_trend,image_pull_backoffs,previous_incidents,label",
                "3,600m,1Gi,Pending,30m,10m,NotReady,4,7,0.5,25,18,2,1,1",
            ]
        ),
        encoding="utf-8",
    )
    frame = build_feature_frame(pd.read_csv(data_path))

    row = frame.iloc[0].to_dict()
    assert row["restarts"] == 3
    assert row["cpu_milli"] == 600
    assert row["memory_mi"] == 1024
    assert row["status_pending"] == 1
    assert row["pod_age_minutes"] == 30
    assert row["phase_duration_minutes"] == 10
    assert row["node_not_ready"] == 1
    assert row["warning_events"] == 4
    assert row["namespace_warning_events"] == 7
    assert row["namespace_non_running_ratio"] == 0.5
    assert row["cpu_trend_delta"] == 25
    assert row["memory_trend_delta"] == 18
    assert row["image_pull_backoff_events"] == 2
    assert row["previous_incidents"] == 1


def test_validate_evaluation_gates_blocks_promotion() -> None:
    """Promotion gates fail model builds when a required metric is too low."""

    gates = promotion_gate_thresholds(min_precision=0.7, min_recall=0.8, min_roc_auc=0.9)

    with pytest.raises(ValueError, match="evaluation gates failed"):
        validate_evaluation_gates({"precision": 0.65, "recall": 0.85}, gates)


def test_best_f1_threshold_prefers_evaluation_fit() -> None:
    """Threshold tuning selects a stronger split point than the default when needed."""

    threshold = best_f1_threshold([0.2, 0.35, 0.55, 0.9], pd.Series([0, 1, 1, 1]))

    assert threshold == 0.35
