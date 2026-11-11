# Python ML service

- Python FastAPI + scikit-learn; training/offline evaluation separate from inference endpoint.
- Start with Isolation Forest on features supported by the actual training dataset; do not promise disease diagnosis or a fixed accuracy.
- Save preprocessing and model version together; inference must validate schema and return `insufficient_data` where appropriate.
- Never fabricate ground-truth labels from synthetic anomaly flags when reporting clinical performance.
- Time-based/entity-aware train/validation split to avoid leakage. Document data provenance and rights.
- Interface in `docs/api.md`, feature policy in `docs/ml.md`; do not add unspecified features to live model input.
- No retraining in an HTTP request; no acceptance of arbitrary user-supplied pickle/joblib artifacts.
- Add tests for missing fields, window validity, version mismatch and deterministic outputs for a pinned artifact.
