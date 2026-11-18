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

## Implemented v1 details

Machine-readable contract: `docs/openapi.json`; deterministic builder: `python3 scripts/generate_openapi.py`. Frontend types: `npm run generate:api` in frontend. This is the first implementation; there are no existing deployed consumers to migrate. Update the builder, generated JSON and generated TypeScript together.

- Lists `/cows`, `/alerts`, `/users`, `/simulations`, predictions and health-summary return `{items,page,page_size,total}`. Page defaults 1/25, max page_size 100. Metric registry and aggregated chart series return arrays.
- `GET /metrics` returns code, Russian name, unit and technical validation bounds. These are not medical reference intervals.
- `GET /simulations` lists runs. `POST /simulations` takes `{farm_id,config}` and returns 202 `{id,state}`; actions return 202 after command storage, actual state changes asynchronously. PATCH config takes a complete configuration; cow_count and mode cannot change live. Replay is a bounded inline array of 1–1000 `{cow_id,metric_type,value,recorded_at}` records, playback_speed in (0,100]. No file upload.
- Login requires the configured Origin and sets an HttpOnly SameSite=Strict session cookie. `/auth/me` returns `{id,username,role,csrf_token,farms}`. Every authenticated mutation requires `X-CSRF-Token` plus matching Origin. Session lifetime 12 hours; Secure cookie configurable for HTTPS.
- `GET /alerts/{id}` includes prediction and action history. `PATCH /users/{id}` changes role, active and full farm list; `POST /users/{id}/password` takes `{password}` and revokes sessions. Scope changes also revoke sessions. Last active admin cannot be disabled.
- Manual POST bodies use `{cow_id,observed_at,payload}`; `observed_at` is the unified API name for actual measurement/sample/inspection time, distinct from created_at. `Idempotency-Key` is required. Reuse for another payload returns 409.
- Manual `GET /{kind}/{id}`, `PATCH /{kind}/{id}`, `GET /{kind}/{id}/revisions` exist for all five kinds. PATCH includes `{cow_id,observed_at,payload,version,reason}`. It creates a revision, preserving original actor/time. Stale version returns 409. `health-summary` returns a combined paginated timeline: `{entry_type: manual|prediction|alert, event_at, data}`. `data` is the corresponding typed resource. Separate `/predictions` and `/alerts` views remain available.
- Milking payload: yield_l, flow_l_min, temperature_c, conductivity_fl/fr/rl/rr, optional weight_kg. Inspection: appetite (`обычный|снижен|отсутствует`), gait (`без замечаний|изменена|затруднена`), condition (`без замечаний|требует внимания|требует осмотра`), optional notes. Lab: test (`bhb|calcium|scc|fat|protein|lactose`), numeric value, matching unit. BCS: bcs 1–5 step .25, hoof category. Veterinary: reproductive_status, notes, optional rectal_temperature_c. Unknown payload fields and wrong numeric types are rejected.
- History query: metric_type, optional cow_id/farm_id, from/to (RFC3339). Default hour; max 90 days; up to ~1000 bins with `{recorded_at,value,min,max,count}`. Half-open interval [from,to); empty bin value/min/max null. `/analytics/milk_yield_l` sums manual yield per bin; other analytics metrics aggregate telemetry. Arbitrary cow groups are not implemented.
- Cow latest metric includes value, unit, source, recorded_at, received_at, stale. Stale after 3 configured intervals (fallback interval 5s). Analysis status is independent. `analysis_delayed` reports a failed pending job; previous completed result remains visible.
- Typed errors use 401/403/404/409/422/429/503. Health endpoints are unauthenticated operational exceptions outside /api/v1.
- WS uses `?farm_id=` as initial scoped subscription; no client subscription messages. Server sends a JSON array of event envelopes every 500ms, max 1000 buffered events. Each connection has an exclusive broker queue (single-host MVP). Session/scope rechecked every 15s. No replay/delivery guarantee; reconnect refetches REST. `telemetry.saved.v1` is exposed as `telemetry.update.v1`; prediction.created, alert.created/updated, simulation.status_changed and manual_measurement.created also invalidate snapshots.
- FastAPI `/predict` additionally requires window_start; v1 window duration is exactly one hour, timezone required. Missing features or unsupported schema return insufficient_data; malformed/non-finite features return 422. Source is internal trusted pipeline only.
