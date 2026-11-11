# Architecture (MVP)

## Components
- React SPA -> Nginx -> Go `api-service` (REST, session auth, role checks, WebSocket).
- Go `sensor-simulator` emits synthetic or timestamp-preserving replay telemetry at configurable rates.
- Go `ingest-service` validates incoming events and publishes to RabbitMQ with confirms.
- Go `data-worker` consumes telemetry, stores historical and latest measurements in PostgreSQL, emits committed events through an outbox.
- Go `ml-analyzer` builds windowed features and invokes Python `ml-service` via FastAPI; saves predictions and creates/deduplicates alerts.
- `api-service` subscribes to committed events and broadcasts scoped, batched updates over WebSocket.
- Manual measurements flow React -> Go API -> PostgreSQL -> async analysis event, not via the simulator.

## Service boundaries
| Service | Writes | Reads / calls |
|---|---|---|
| api-service | users/sessions, animals, manual records, alert actions, simulations/commands | Postgres, RabbitMQ, WS clients |
| sensor-simulator | telemetry events (HTTP) | simulation commands / config |
| ingest-service | broker messages | HTTP sensor input |
| data-worker | sensor history, latest state, outbox | RabbitMQ |
| ml-analyzer | predictions, alerts, outbox | Postgres windows, Python ML service |
| ml-service | no DB writes in MVP | HTTP inference from analyzer |

All services may share one PostgreSQL deployment in MVP, but each writes only its owned tables. Schema migration ownership is explicit. Keep RabbitMQ event envelopes versioned. Request processing must not wait on ML.

## Telemetry delivery
1. Simulator POSTs batch/event to ingest (auth, schema, rate limits).
2. Ingest publishes persistent message and waits for broker confirm; 202 means accepted by broker only.
3. Worker consumes, commits measurement and outbox row in one DB transaction; dedupe by `event_id`; ACK only after commit.
4. Outbox publisher emits `telemetry.saved.v1`; API hub receives and updates subscribed browsers.
5. Analyzer schedules analysis by cow and time window, independent of ingestion, and persists prediction/alert.
6. API broadcasts `prediction.created.v1` / `alert.created.v1` after commit.

## Failure behavior
- ML down: telemetry storage unaffected; queue analysis work/retry boundedly, expose status `pending` or `insufficient_data`.
- Broker down: ingest returns failure/backpressure, never claims accepted; simulator retries idempotently.
- WebSocket down: clients refetch snapshots on reconnect; no claim of guaranteed browser delivery.
- Out-of-order events: history stores `recorded_at`; latest state only updated by newer observation.

## Identity
Only `admin` and `operator`. Server enforces role and farm scope on REST and WS. Admin manages users/animals/simulation. Operator views data, adds/corrects manual measurements, acknowledges/resolves alerts. Server-side sessions in Postgres and secure HttpOnly cookies.
