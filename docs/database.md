# PostgreSQL outline (logical, not a finished migration)

## Identity and cattle
`users` (id, username, password_hash, role `admin|operator`, farm scope), `sessions` (opaque token hash, user_id, expiry), `farms`, `cows`, `sensors`, `metric_definitions` (code, unit, validation metadata).

## Telemetry
`sensor_measurements` (event_id UNIQUE, simulation_id, cow_id, sensor_id, metric_type, value, recorded_at, received_at, source `synthetic|replay`); index `(cow_id, metric_type, recorded_at DESC)` and time index; DB check constraints where useful. `cow_latest_state` keyed by `(cow_id, metric_type)` and updated ONLY for newer `recorded_at`.

## Manual records
`milking_records` and `milking_quarter_conductivity`; `visual_inspections`; `laboratory_tests` + `laboratory_test_values` (typed test/value/unit); `body_condition_records`; `veterinary_examinations`. Every record has `measured_at`/`observed_at`, `created_at`, `created_by`, and revision/audit association.

## Analysis, controls, events
`ml_predictions` (cow_id, window_start/end, status, score, model_version, schema_version, created_at); `alerts` (cow_id, state, observed window, dedupe key, last_seen), `alert_actions`; `simulations`, `simulation_commands`; `outbox_events` (unique id, event_type, payload, published_at); `audit_log`.

## Integrity decisions
- No clinical diagnosis column derived solely from unsupervised ML.
- Alert dedupe for same active cow/condition/window; separate history of detections.
- Use parametrized SQL and migrations with explicit down/rollback plan.
- Retention and partitioning are measured decisions; do not introduce TimescaleDB prematurely.
- Split database writers by table ownership documented in `docs/architecture.md`.
