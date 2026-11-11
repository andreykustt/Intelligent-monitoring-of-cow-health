# Implementation roadmap (one milestone per agent session)

| Milestone | Deliverable | Exit check |
|---|---|---|
| M0 | Confirm contracts, metrics, two roles and product boundaries | docs aligned; no ambiguous units |
| M1 | Repo + Compose + Postgres migrations + RabbitMQ + Nginx + React/Go skeleton | stack starts; readiness checks; API returns auth identity |
| M2 | Simulator + ingest + RabbitMQ + worker + storage | 200 cows generate; readings saved idempotently |
| M3 | REST snapshots/history + WebSocket + React monitor | live table updates; filter/sort server-side; reconnect resync |
| M4 | Manual forms, audit, cow profile | milking/lab/inspection saved and visible in timeline |
| M5 | ML offline training + API + analyzer + alerts | anomalous pattern generates alert; ML outage doesn't stop ingest |
| M6 | Analytics, RBAC, simulation admin console | correct role gating, charts with server aggregation |
| M7 | Integration, load tests, user documentation, demo | acceptance scenarios pass and limits documented |

## Work split
- Backend: Go API, auth, manual records, SQL; IoT: simulator/ingest; ML: datasets/model/Python; DevOps: Compose/RabbitMQ/Nginx/CI; BI: analytics/dashboard specs and implementation support; R&D: metric definitions, research, UX and acceptance tests.
- Nominate one accountable React implementer; do not assume BI analyst alone can deliver a full frontend.

## Agent session workflow
1. Pick ONE milestone and ONE bounded task.
2. Read root AGENTS, relevant local AGENTS, and ONLY the contract docs required.
3. Write a brief file-change plan, implement, run affected checks.
4. Record what was built, changed contracts, tests and blockers in `docs/progress.md`.
5. Start a new agent session for another bounded task; keep instructions stable during a session.
