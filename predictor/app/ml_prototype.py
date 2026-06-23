"""Optional ML model training utility for the predictor service.

The predictor service remains deterministic by default. This module trains a
small scikit-learn model that can be loaded at runtime by setting
``PREDICTOR_MODEL_PATH`` to the saved joblib file.
"""

from __future__ import annotations

import argparse
from pathlib import Path

import joblib
import pandas as pd
from sklearn.ensemble import RandomForestClassifier

FEATURE_COLUMNS = ("restarts", "cpu_milli", "memory_mi")
LABEL_COLUMNS = ("failure_imminent", "label", "target")


def parse_cpu_milli(value: object) -> float:
    """Parse CPU values to milli-CPU units."""

    raw = str(value).strip().lower()
    if raw == "" or raw == "nan" or raw == "n/a":
        return 0.0
    if raw.endswith("m"):
        return float(raw[:-1] or 0)
    return float(raw) * 1000


def parse_memory_mi(value: object) -> float:
    """Parse memory values to mebibytes."""

    raw = str(value).strip().lower()
    if raw == "" or raw == "nan" or raw == "n/a":
        return 0.0
    if raw.endswith("mi"):
        return float(raw[:-2] or 0)
    if raw.endswith("gi"):
        return float(raw[:-2] or 0) * 1024
    if raw.endswith("ki"):
        return float(raw[:-2] or 0) / 1024
    if raw.endswith("b"):
        return float(raw[:-1] or 0) / (1024 * 1024)
    return float(raw)


def choose_column(frame: pd.DataFrame, candidates: tuple[str, ...]) -> str:
    """Return the first present column from a candidate list."""

    for column in candidates:
        if column in frame.columns:
            return column
    raise ValueError(f"missing required column; expected one of: {', '.join(candidates)}")


def build_feature_frame(frame: pd.DataFrame) -> pd.DataFrame:
    """Normalize historical pod rows into the service feature contract."""

    restarts_col = choose_column(frame, ("restarts", "restart_count"))
    cpu_col = choose_column(frame, ("cpu_milli", "cpu", "cpu_usage"))
    memory_col = choose_column(frame, ("memory_mi", "memory", "memory_usage"))

    return pd.DataFrame(
        {
            "restarts": pd.to_numeric(frame[restarts_col], errors="coerce").fillna(0),
            "cpu_milli": frame[cpu_col].map(parse_cpu_milli).fillna(0),
            "memory_mi": frame[memory_col].map(parse_memory_mi).fillna(0),
        },
        columns=FEATURE_COLUMNS,
    )


def build_labels(frame: pd.DataFrame) -> pd.Series:
    """Normalize target labels to integer 0/1 values."""

    label_col = choose_column(frame, LABEL_COLUMNS)
    raw = frame[label_col]
    if pd.api.types.is_bool_dtype(raw):
        return raw.astype(int)
    if pd.api.types.is_numeric_dtype(raw):
        return raw.fillna(0).astype(int).clip(0, 1)

    truthy = {"1", "true", "yes", "failure", "failure_imminent", "failed", "risk"}
    return raw.astype(str).str.strip().str.lower().isin(truthy).astype(int)


def train_simple_model(data_path: str, model_save_path: str) -> Path:
    """Train and save a pod failure classifier from a CSV file.

    Expected input columns:
        - restarts or restart_count
        - cpu_milli, cpu, or cpu_usage
        - memory_mi, memory, or memory_usage
        - failure_imminent, label, or target

    Args:
        data_path: CSV file with historical pod observations.
        model_save_path: Destination joblib path.

    Returns:
        Path: Saved model path.
    """

    frame = pd.read_csv(data_path)
    features = build_feature_frame(frame)
    labels = build_labels(frame)

    if labels.nunique() < 2:
        raise ValueError("training data must contain both positive and negative labels")

    model = RandomForestClassifier(
        n_estimators=200,
        max_depth=8,
        min_samples_leaf=2,
        class_weight="balanced",
        random_state=42,
    )
    model.fit(features.to_numpy(), labels.to_numpy())

    output = Path(model_save_path)
    output.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump(model, output)
    return output


def main() -> None:
    """CLI entrypoint for model training."""

    parser = argparse.ArgumentParser(description="Train an optional KubeLens predictor ML model.")
    parser.add_argument("data_path", help="CSV file with historical pod observations")
    parser.add_argument("model_save_path", help="Output joblib model path")
    args = parser.parse_args()

    saved_path = train_simple_model(args.data_path, args.model_save_path)
    print(f"Model saved to {saved_path}")


if __name__ == "__main__":
    main()
