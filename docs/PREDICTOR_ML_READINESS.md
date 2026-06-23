# Predictor ML Readiness Plan

This document tracks the work required to move the optional predictor ML path from basic model blending to production-grade incident prediction.

## Current state

- Deterministic scoring remains the default predictor behavior.
- Optional ML scoring is enabled only when `PREDICTOR_MODEL_PATH` points to a loadable joblib model.
- Runtime feature contract is currently `restarts`, `cpu_milli`, and `memory_mi`.
- ML scores can raise deterministic pod risk, but cannot lower deterministic risk.
- `predictor/app/ml_prototype.py` trains a CSV-backed random forest model compatible with the runtime feature contract.

## Production target

The ML module should be explainable, observable, reproducible, and safe to run in production. Model output must improve prioritization without hiding deterministic evidence or producing unbounded false positives.

## Feature roadmap

| Area              | Required work                                                                                                                                                                                                          | Status  |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- |
| Feature set       | Add pod status encoding, age parsing, warning event counts, namespace pressure, node readiness, restart velocity, CPU and memory trends, pod phase duration, image pull/backoff signals, and previous incident labels. | Planned |
| Training pipeline | Promote the trainer into a versioned pipeline with train/validation/test splits, model metadata, reproducible seeds, and saved metrics.                                                                                | Planned |
| Calibration       | Add calibrated probabilities or threshold tuning so risk scores map to operational confidence.                                                                                                                         | Planned |
| Evaluation gates  | Fail CI/model promotion when recall, precision, false-positive rate, or calibration falls outside defined bounds.                                                                                                      | Planned |
| Shadow mode       | Support emitting ML scores without blending them into final risk during rollout.                                                                                                                                       | Planned |
| Runtime safety    | Weight ML influence by feature completeness, model health, and data freshness.                                                                                                                                         | Planned |
| Observability     | Export model version, inference latency, load failures, feature missing rates, score distribution, drift signals, and ML/deterministic disagreement.                                                                   | Planned |
| Packaging         | Rename the trainer to a production-oriented entrypoint, document CSV schema, add fixtures, and separate optional ML dependencies from default runtime.                                                                 | Planned |

## Model metadata contract

Every promoted model artifact should have a sidecar metadata file with:

- model version
- git commit
- training data window
- feature list and ordering
- label definition
- evaluation metrics
- calibrated threshold
- training timestamp
- owner/reviewer

## Rollout policy

1. Train and evaluate offline.
2. Run in shadow mode.
3. Compare ML risk against deterministic risk and incident outcomes.
4. Enable blending with low ML weight.
5. Increase ML weight only when drift and false-positive metrics remain healthy.

## Safety rules

- Deterministic risk remains the floor.
- Missing or malformed model artifacts disable ML scoring.
- Low feature completeness reduces ML influence.
- ML disagreement should be surfaced as a signal, not hidden.
- Operators must be able to identify which model version produced a score.
