# Go services

Applies to Go code under `services/` (Python ML has its own more specific rules).
- Services: `api-service`, `ingest-service`, `data-worker`, `ml-analyzer`, `sensor-simulator`.
- Keep each service's `cmd/` entrypoint thin; business logic in `internal/`.
- The API service owns HTTP auth, RBAC, manual records, read endpoints and WebSocket hub.
- Ingest validates telemetry, publishes to RabbitMQ, confirms broker receipt; it does not run ML or write DB.
- Worker persists idempotently and publishes `telemetry.saved` via transactional outbox after commit.
- Analyzer calculates features using only available timestamped observations; calls Python inference asynchronously.
- Simulator supports cow count, per-metric frequency, replay, synthetic anomalies, pause/resume/stop, rate limits; it must not create alerts itself.
- Propagate context/deadlines; handle graceful shutdown, retry bounds and poison messages.
- Use explicit SQL migrations; no schema changes automatically at runtime.
- Test authorization, event idempotency, event ordering, bad payloads and out-of-date measurements.
- Read `docs/data-contracts.md` for metrics; `docs/api.md` for routes; `docs/database.md` for tables.
