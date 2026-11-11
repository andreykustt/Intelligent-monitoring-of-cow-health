# API contract (proposed v1; formalize as OpenAPI during phase 1)

Base: `/api/v1`. JSON, UTC ISO 8601 timestamps, pagination and typed errors. Cookies use server sessions. All HTTP methods except login require authorization. Admin-only routes below marked `[admin]`.

## Core endpoints
| Method | Path | Purpose |
|---|---|---|
| POST | `/auth/login` | login |
| POST | `/auth/logout` | logout |
| GET | `/auth/me` | session identity/role |
| GET | `/cows` | paginated list: `farm_id`, `status`, `q`, `sort`, `page`, `page_size` |
| GET | `/cows/{id}` | details + latest state |
| POST/PATCH | `/cows` / `/cows/{id}` | animal management `[admin]` |
| GET | `/cows/{id}/measurements` | time-filtered history with `metric_type` |
| GET | `/cows/{id}/predictions` | ML history incl. model version |
| GET | `/telemetry/latest` | latest per cow/metric; paging/filtering |
| GET | `/telemetry/history` | server-side time window and downsampling |
| GET | `/analytics/overview` | herd overview |
| GET | `/analytics/{metric}` | aggregated bins by date, farm, cow group |
| GET | `/alerts` | paginated alerts |
| POST | `/alerts/{id}/acknowledge` | acknowledge |
| POST | `/alerts/{id}/resolve` | resolve with note |
| POST | `/milkings` | manual milking record |
| POST | `/inspections` | visual inspection |
| POST | `/laboratory-results` | lab result(s) |
| POST | `/body-condition-records` | BCS/hoof record |
| POST | `/veterinary-examinations` | exam record |
| GET | `/cows/{id}/health-summary` | combined timeline/current picture |
| POST | `/simulations` | start `[admin]` |
| GET | `/simulations/{id}` | progress `[admin]` |
| POST | `/simulations/{id}/pause` | pause `[admin]` |
| POST | `/simulations/{id}/resume` | resume `[admin]` |
| POST | `/simulations/{id}/stop` | stop `[admin]` |
| PATCH | `/simulations/{id}/config` | change live rate `[admin]` |
| GET/POST | `/users` | manage users `[admin]` |

Use consistent error: `{ "error": {"code":"VALIDATION_ERROR","message":"...","details":{}} }`. Validate filter/sort allowlists, date ranges and maximum page/window sizes server-side. Manual edit should create an auditable revision, not erase originals.

## WebSocket
`GET /api/v1/stream` (upgrade, session auth, origin validation, farm scope). Initial data comes from REST. Client subscribes with permitted filters; server emits batched `telemetry.update.v1`, `cow.status_changed.v1`, `alert.created.v1`, `simulation.status_changed.v1`. Event envelope: `type`, `event_id`, `occurred_at`, `data`. Client reconnects with exponential backoff and REST snapshot refresh. WS is best-effort and not an authoritative history source.

## Internal contracts
- `POST /internal/v1/telemetry` -> 202 only after broker confirm; 503/retryable on broker failure.
- `POST ml-service:/predict` -> `cow_id`, `window_end`, `features`, `feature_schema_version`; returns `status: anomaly|no_anomaly|insufficient_data`, `anomaly_score` (nullable), `model_version` and optional reasons if legitimately supported. A score is not disease probability.
- RabbitMQ events: `telemetry.received.v1` (ingest), `telemetry.saved.v1` (outbox), `manual_measurement.created.v1`, `prediction.created.v1`, `alert.created.v1`, `simulation.command.v1`.

Contract changes must update producer, consumer, tests and docs in the same change.
