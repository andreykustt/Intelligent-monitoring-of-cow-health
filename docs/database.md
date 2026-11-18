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

## Implemented physical schema

`infrastructure/migrations/001_initial.up.sql` is the first explicit schema. The Go services share one Go module but retain separate cmd entrypoints and internal ownership. user_farms implements many-to-many scope. Cow IDs and farm IDs are text; operational/event IDs are UUID. sessions stores SHA-256 hashes of opaque random tokens; passwords use bcrypt cost 12.

Manual physical storage uses `manual_records(kind,payload jsonb)` and append-only `manual_revisions` rather than five normalized tables from the initial outline. This bounded MVP choice shares revision/idempotency logic; API validates kind-specific numeric types, enums, units and required fields. Historical measurements stay typed relational rows; no measurements are stored in an unvalidated generic blob. Queries/consumers use the documented API, so the physical change has no existing consumer migration.

Analysis jobs are durable and retried independently. Prediction uniqueness is (cow_id,window_end,model_version,schema_version). Active alert uniqueness is (cow_id,condition) WHERE state!='resolved'. Outbox rows have an owner; each owning service publishes only its rows. Simulator reads durable simulation commands/configs and publishes actual state; API consumes state events and writes simulations. Command events are wakeup/integration notifications; database command versions are the durable reconciliation source. API owns alert_actions and changes alert state as its documented exception to analyzer ownership.

Unknown/invalid worker messages are dead-lettered; transient processing errors are requeued with quorum delivery limit 5. Delivery is at-least-once. No automatic retention/partitioning. Migration rollback SQL is explicit and destructive for the initial schema; use backup and operator review.
