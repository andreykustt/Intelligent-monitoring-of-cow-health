# Data definitions & provenance

## Automatic telemetry (simulator only in MVP)
Envelope (UTC ISO 8601; `recorded_at` may be in the historical past):
```json
{
  "schema_version": 1,
  "event_id": "<uuid>",
  "simulation_id": "<uuid>",
  "cow_id": "COW-001",
  "sensor_id": "SIM-TEMP-001",
  "recorded_at": "2026-11-11T15:30:00Z",
  "metric_type": "body_temperature_c",
  "value": 38.6
}
```
One event = one scalar reading. Batch HTTP ingestion may group events; each retains own `event_id`. Deduplicate by ID. Preserve `received_at` on ingest/storage. Explicitly separate simulator source `synthetic` vs `replay`.

| Key | Unit | Description |
|---|---|---|
| `body_temperature_c` | °C | Simulated body temperature |
| `respiratory_rate_bpm` | breaths/min | Breathing rate; `bpm` refers to breaths here |
| `heart_rate_bpm` | beats/min | Pulse |
| `activity_index` | 0–1 (defined index) | Relative activity, algorithm documented |
| `steps_per_min` | steps/min | Movement |
| `rumen_ph` | pH | Rumen pH |
| `rumination_min_1h` | min per preceding hour | Rumination duration |
| `feed_intake_kg_1h` | kg per preceding hour | Food intake |
| `water_intake_l_1h` | L per preceding hour | Water intake |

All simulator frequencies and value ranges are configurable, bounded, and documented; reference distributions must not be represented as medical norms. For hourly-window metrics, timestamps specify window end. A metric can be missing/stale independently.

## Manual forms (not automatically sampled)
- `milking`: `measured_at`, yield (L), flow (L/min), milk temperature (°C), conductivity per udder quarter (mS/cm), optional live weight (kg) if measured then.
- `visual_inspection`: observed_at, appetite category, gait category, general-condition category, free text.
- `laboratory_result`: sample_at, analysis type, one or more parameter keys and units; e.g., BHB (mmol/L), calcium (mmol/L), SCC (cells/mL), fat/protein/lactose (%); don't mix unrelated tests in one required-field form.
- `body_condition`: observed_at, BCS score with specified scale, hoof assessment.
- `veterinary_examination`: observed_at, rectal temp (°C, optional), reproductive status, notes.

Persist `measured_at`/`observed_at` vs `created_at`, `created_by`, correction audit and provenance. Manual entries may be backdated; do not rewrite earlier ML outcomes silently. No clinical diagnosis is inferred from synthetic telemetry.

## Simulation configuration
- `cow_count`, `metrics_enabled`, `interval_ms_by_metric`, `duration_seconds`, `mode: synthetic|replay`, `playback_speed`, `scenario`, optionally `target_events_per_second` (advanced).
- Distinguish number of cows, number of sensors, readings per sensor, HTTP request rate, and broker event rate.
- States `created`, `starting`, `running`, `paused`, `completed`, `stopped`, `failed`. Commands idempotent/versioned.

## Implementation additions

Telemetry requires `source: synthetic|replay` explicitly. Ingest sets received_at in UTC before publication and overwrites any client-supplied arrival time. Retries retain event_id; a previously stored duplicate preserves its original received_at. Sensor IDs in v1 are `<cow_id>:<metric_type>`. The worker validates registration and farm relationship to simulation; invalid foreign references go to DLQ. Ingest performs envelope/technical-range validation without writing or querying PostgreSQL; 202 is broker acceptance, not storage validation/commit.

Manual API uses observed_at consistently for all kinds; UI labels it as measurement/sample/inspection time. Payload fields and units are specified in api.md. Audit preserves original created_by and created_at; each revision stores its own actor/time/reason. BCS uses scale 1–5 step .25. Hourly telemetry remains synthetic scalar windows; activity is a generated dimensionless value in [0,1], not an estimate computed from real accelerometers.

Simulator defaults: 200 registered cows, all 9 metrics, 5000ms each, duration 7200s, seed 42. Technical intervals 1000–3600000ms, max 1000 cows, max 24h duration. Synthetic bases: temperature 38.6, pulse 65, respiration 25, activity .5, steps 12, pH 6.4, rumination 25, feed 2, water 4; independent Gaussian noise std=2.5% of base, clamped nonnegative. These values are demo settings, not medical norms. Anomaly scenario changes temperature/pulse/activity for every tenth cow; missing scenario skips every fifth cow. Per-metric frequency is configurable; distribution customization via UI is deferred.
