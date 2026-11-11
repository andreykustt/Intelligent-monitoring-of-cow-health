# ML scope and validity

## MVP goal
Detect unusual patterns in cow telemetry, not diseases. Start with Isolation Forest on real, legally reusable open datasets if possible. Synthetic data is for integration/demo; model performance measured on synthetic scenarios is NOT clinical sensitivity/specificity.

## Offline workflow
1. Record dataset URL, license, columns, animal count, timestamp coverage, missingness in dataset card.
2. Inspect per-cow time series and temporal resolution; choose ONLY genuinely present features.
3. Aggregate windows and build per-animal baselines carefully; store feature schema and missing-data behavior.
4. Split by time and, where possible, animal; prevent overlapping window leakage.
5. Evaluate anomalous pattern detection on held-out data; report limitations, false alerts, reproducibility.
6. Export a versioned preprocessing+model artifact from trusted training pipeline.

## Inference workflow
- Go `ml-analyzer` schedules analysis independently from ingest, computes valid windows, sends versioned feature payload to Python `/predict`.
- Python returns one of `anomaly`, `no_anomaly`, `insufficient_data`. Score is NOT probability of illness.
- Manual lab/milking data initially contributes to trend/rule analysis, not automatically to the trained model; introduce a multimodal model only if adequate aligned data exists.
- Validate data freshness: do not merge a two-week-old lab result with current vitals as if simultaneous.
- Repeated unusual readings should update/dedupe alerts rather than emit endless new ones.
- Track evaluation and model version in docs; never promise predefined accuracy without evidence.
