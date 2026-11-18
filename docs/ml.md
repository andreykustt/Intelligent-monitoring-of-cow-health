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

## Implemented educational baseline

The checked-in training script uses synthetic feature windows only. No claim is made that a real dataset has been found or that clinical validation occurred. Replacing this baseline with legally reusable real data remains a separate research task.

Schema 1 features, ordered: mean/stddev/linear slope per minute for body_temperature_c, heart_rate_bpm, activity_index. Analyzer aggregates each metric into minute means over the preceding 60 minutes; each needs at least 48 covered minute bins. Five-minute window ends; absent features produce insufficient_data. Late manual records do not rewrite prior predictions. ML outages leave durable jobs pending with bounded 30s retry delay.

`train.py` creates 2000 independent synthetic feature windows with seed 42, uses first 1600 for training and last 400 for held-out reporting. This is a feature-level educational baseline, not a real animal/time-series validation. Pipeline: StandardScaler + IsolationForest, 150 estimators, contamination .03, random_state 42, single-threaded. Trusted build creates the artifact; HTTP cannot supply models or trigger training. Model version synthetic-iforest-v1. Report in the model image records held-out flag fraction .0275 for the pinned pipeline; this is not sensitivity/specificity or disease probability.

Known limit: finalized windows are immutable; subsequent historical telemetry does not automatically recalculate them. Replay should be sent chronologically; scheduling uses a short arrival settling delay. No multimodal training or medically supported explanations.
