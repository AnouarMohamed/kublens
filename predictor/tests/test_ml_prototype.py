"""Optional predictor ML trainer tests."""

import json

import pytest

pytest.importorskip("joblib")
pytest.importorskip("pandas")
pytest.importorskip("sklearn")

from predictor.app.ml_prototype import train_simple_model


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
    assert payload["featureList"] == ["restarts", "cpu_milli", "memory_mi"]
    assert payload["labelDefinition"] == "label=incident within rollout horizon"
    assert payload["ownerReviewer"] == "sre-platform"
    assert payload["calibratedThreshold"] == 0.5
    assert "2026-07-01T00:00:00+00:00/2026-07-01T00:35:00+00:00" == payload["trainingDataWindow"]
    assert "precision" in payload["evaluationMetrics"]
    assert "recall" in payload["evaluationMetrics"]
