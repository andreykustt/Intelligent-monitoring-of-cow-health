# MVP acceptance tests

1. Admin logs in, creates a synthetic simulation with chosen cow count and per-metric intervals, starts/pauses/resumes/stops it.
2. Operator cannot create simulations or manage users even by calling API directly.
3. Events appear in PostgreSQL with correct cow, metric, unit via registry, simulation ID, event time and receipt time; duplicate event IDs do not duplicate rows.
4. React monitor streams new values without reload and supports server-side sorting, filtering and pagination; bounded event feed remains responsive under increased rate.
5. Cow detail shows last known values and historical plots with source and freshness labels.
6. Operator manually enters milking (quarter conductivity), visual inspection, lab test and BCS/hoof assessment; timestamps, actor and correction history survive reload.
7. ML produces `anomaly` or `no_anomaly` only on supported features; missing history yields `insufficient_data`.
8. Trigger synthetic scenario and demonstrate warning; explicitly disclose synthetic nature and lack of diagnostic certainty.
9. Stop ML service: telemetry ingest/history still works; recover and process pending analyses.
10. Stop/restart browser WebSocket: reconnect refreshes REST snapshot and no false guarantee of total event delivery.
11. Stop broker: ingest doesn't falsely return success; simulator retries with stable event IDs.
12. Deploy with `docker compose up -d --build`, without Grafana, Prometheus, Redis or Kubernetes; no secrets committed.

## Performance test plan (targets, not claims)
- Baseline 200 cows with configured metrics; 1000 cows extended; document messages/s, HTTP RPS, DB writes/s, p95 API and WS lag.
- Load test only on controlled environment; report measured hardware, payload size, metric frequency and failures.
- No promised RPS until tests are performed.
