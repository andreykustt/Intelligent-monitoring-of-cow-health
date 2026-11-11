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
