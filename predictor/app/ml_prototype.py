"""Optional ML model training utility for the predictor service.

The predictor service remains deterministic by default. This module trains a
small scikit-learn model that can be loaded at runtime by setting
``PREDICTOR_MODEL_PATH`` to the saved joblib file.
"""

from __future__ import annotations

import argparse
import json
import os
from datetime import datetime, timezone
from pathlib import Path

import joblib
import pandas as pd
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import accuracy_score, precision_score, recall_score, roc_auc_score
from sklearn.model_selection import train_test_split

FEATURE_COLUMNS = ("restarts", "cpu_milli", "memory_mi")
LABEL_COLUMNS = ("failure_imminent", "label", "target")
TIME_COLUMNS = ("timestamp", "observed_at", "created_at")
DEFAULT_THRESHOLD = 0.5


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

    labels, _ = build_labels_with_column(frame)
    return labels


def build_labels_with_column(frame: pd.DataFrame) -> tuple[pd.Series, str]:
    """Normalize target labels and return the selected source column."""

    label_col = choose_column(frame, LABEL_COLUMNS)
    raw = frame[label_col]
    if pd.api.types.is_bool_dtype(raw):
        return raw.astype(int), label_col
    if pd.api.types.is_numeric_dtype(raw):
        return raw.fillna(0).astype(int).clip(0, 1), label_col

    truthy = {"1", "true", "yes", "failure", "failure_imminent", "failed", "risk"}
    return raw.astype(str).str.strip().str.lower().isin(truthy).astype(int), label_col


def default_metadata_path(model_path: Path) -> Path:
    """Return the default sidecar path for a trained model."""

    return model_path.with_name(f"{model_path.stem}.metadata.json")


def training_data_window(frame: pd.DataFrame) -> str:
    """Return a concise training window from timestamp-like CSV columns."""

    for column in TIME_COLUMNS:
        if column not in frame.columns:
            continue
        parsed = pd.to_datetime(frame[column], errors="coerce", utc=True).dropna()
        if parsed.empty:
            continue
        return f"{parsed.min().isoformat()}/{parsed.max().isoformat()}"
    return f"rows={len(frame)}"


def evaluation_metrics(
    model: RandomForestClassifier,
    features: pd.DataFrame,
    labels: pd.Series,
    threshold: float,
) -> dict[str, float]:
    """Evaluate the trained model with stable binary classification metrics."""

    probabilities = model.predict_proba(features.to_numpy())[:, 1]
    predictions = (probabilities >= threshold).astype(int)
    metrics = {
        "accuracy": round(float(accuracy_score(labels, predictions)), 4),
        "precision": round(float(precision_score(labels, predictions, zero_division=0)), 4),
        "recall": round(float(recall_score(labels, predictions, zero_division=0)), 4),
    }
    if labels.nunique() > 1:
        metrics["rocAuc"] = round(float(roc_auc_score(labels, probabilities)), 4)
    return metrics


def write_model_metadata(
    *,
    metadata_path: Path,
    model_version: str,
    git_commit: str,
    window: str,
    label_column: str,
    metrics: dict[str, float],
    threshold: float,
    owner_reviewer: str,
) -> Path:
    """Write the runtime model metadata sidecar."""

    metadata_path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "modelVersion": model_version,
        "gitCommit": git_commit,
        "trainingDataWindow": window,
        "featureList": list(FEATURE_COLUMNS),
        "labelDefinition": f"{label_column}=incident within rollout horizon",
        "evaluationMetrics": metrics,
        "calibratedThreshold": threshold,
        "trainingTimestamp": datetime.now(timezone.utc).isoformat(),
        "ownerReviewer": owner_reviewer,
    }
    metadata_path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return metadata_path


def train_simple_model(
    data_path: str,
    model_save_path: str,
    *,
    metadata_save_path: str = "",
    model_version: str = "",
    git_commit: str = "",
    owner_reviewer: str = "",
    calibrated_threshold: float = DEFAULT_THRESHOLD,
) -> Path:
    """Train and save a pod failure classifier from a CSV file.

    Expected input columns:
        - restarts or restart_count
        - cpu_milli, cpu, or cpu_usage
        - memory_mi, memory, or memory_usage
        - failure_imminent, label, or target

    Args:
        data_path: CSV file with historical pod observations.
        model_save_path: Destination joblib path.
        metadata_save_path: Optional destination for the metadata sidecar.
        model_version: Version label written to metadata.
        git_commit: Source commit written to metadata.
        owner_reviewer: Owner/reviewer label written to metadata.
        calibrated_threshold: Probability threshold used for evaluation metrics.

    Returns:
        Path: Saved model path.
    """

    frame = pd.read_csv(data_path)
    features = build_feature_frame(frame)
    labels, label_column = build_labels_with_column(frame)

    if labels.nunique() < 2:
        raise ValueError("training data must contain both positive and negative labels")

    threshold = min(1.0, max(0.0, float(calibrated_threshold)))
    can_stratify = len(labels) >= 8 and labels.value_counts().min() >= 2
    if can_stratify:
        train_features, eval_features, train_labels, eval_labels = train_test_split(
            features,
            labels,
            test_size=0.25,
            random_state=42,
            stratify=labels,
        )
    else:
        train_features, eval_features, train_labels, eval_labels = features, features, labels, labels

    model = RandomForestClassifier(
        n_estimators=200,
        max_depth=8,
        min_samples_leaf=2,
        class_weight="balanced",
        random_state=42,
    )
    model.fit(train_features.to_numpy(), train_labels.to_numpy())

    output = Path(model_save_path)
    output.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump(model, output)

    metadata_output = Path(metadata_save_path) if metadata_save_path.strip() else default_metadata_path(output)
    write_model_metadata(
        metadata_path=metadata_output,
        model_version=model_version.strip() or output.stem,
        git_commit=git_commit.strip() or os.getenv("APP_COMMIT", "local"),
        window=training_data_window(frame),
        label_column=label_column,
        metrics=evaluation_metrics(model, eval_features, eval_labels, threshold),
        threshold=threshold,
        owner_reviewer=owner_reviewer.strip(),
    )
    return output


def main() -> None:
    """CLI entrypoint for model training."""

    parser = argparse.ArgumentParser(description="Train an optional KubeLens predictor ML model.")
    parser.add_argument("data_path", help="CSV file with historical pod observations")
    parser.add_argument("model_save_path", help="Output joblib model path")
    parser.add_argument("--metadata-path", default="", help="Output metadata JSON path")
    parser.add_argument("--model-version", default="", help="Model version label")
    parser.add_argument("--git-commit", default="", help="Source commit for the training code/data contract")
    parser.add_argument("--owner-reviewer", default="", help="Owner or reviewer responsible for promotion")
    parser.add_argument("--calibrated-threshold", type=float, default=DEFAULT_THRESHOLD, help="Evaluation threshold")
    args = parser.parse_args()

    saved_path = train_simple_model(
        args.data_path,
        args.model_save_path,
        metadata_save_path=args.metadata_path,
        model_version=args.model_version,
        git_commit=args.git_commit,
        owner_reviewer=args.owner_reviewer,
        calibrated_threshold=args.calibrated_threshold,
    )
    print(f"Model saved to {saved_path}")
    print(f"Metadata saved to {Path(args.metadata_path) if args.metadata_path else default_metadata_path(saved_path)}")


if __name__ == "__main__":
    main()
