# Cow Health Monitor — agent entry point

## Product boundary
- Educational MVP: synthetic/replayed cattle sensor telemetry + manually entered observations, anomalies, dashboards and alerts.
- Only two roles: `admin` and `operator`. The app must never claim to diagnose disease.
- Stack: Go services, Python ML (FastAPI/scikit-learn), React/TypeScript, PostgreSQL, RabbitMQ, WebSocket, Nginx, Docker Compose.
- No Grafana, Prometheus, Redis, Kubernetes or external hardware in MVP.

## Repo navigation (read only when relevant)
- Architecture and ownership: `docs/architecture.md`.
- HTTP/WebSocket contracts: `docs/api.md`.
- Telemetry and manual measurement units: `docs/data-contracts.md`.
- DB design: `docs/database.md`.
- ML assumptions: `docs/ml.md`.
- Milestones/acceptance: `docs/implementation-plan.md`, `docs/acceptance.md`.
- Local instructions: `services/AGENTS.md`, `services/ml-service/AGENTS.md`, `frontend/AGENTS.md`, `infrastructure/AGENTS.md`.

## Non-negotiable rules
- Do not change documented contracts or schemas silently; update docs, migrations and affected tests together.
- Keep `recorded_at` (measurement time) distinct from `received_at` (arrival) and `created_at` (manual entry); UTC timestamps on the wire.
- Simulated health anomalies are NOT medically verified labels. Show `normal/no anomaly`, `anomaly` and `insufficient_data` distinctly; never label a cow clinically healthy.
- ML inference must not block telemetry ingestion or storage; on ML outage, ingestion continues.
- Use RabbitMQ publisher confirms, consumer acknowledgements and idempotent message handling. Acknowledge ingestion only after broker confirm; never imply DB commit from ingest acceptance.
- Check role permissions on server endpoints and WebSocket subscriptions; never trust hidden UI controls.
- Do not commit credentials, `.env`, production data or large datasets. Use `.env.example`.
- Don't add technologies or microservices beyond the documented scope without an explicit tradeoff and approval.
- Make minimal changes for the current task; do not opportunistically refactor unrelated modules.

## Commands (once corresponding code exists)
- Full stack: `docker compose up -d --build`; stop: `docker compose down` (no `-v` by default).
- Compose validation: `docker compose config -q`.
- Go per service: `go test ./...`, `go vet ./...`, `gofmt -w <changed-go-files>`.
- Python ML: `python -m pytest -q`, `python -m ruff check .` (from ML directory).
- Frontend: `npm ci`, `npm run lint`, `npm run build` (from `frontend/`).
- Commands are targets, not guarantees: add corresponding files/scripts during implementation. Report unavailable checks; never claim they passed.

## Agent completion format
- Summarize touched files, behavior delivered, tests run/results, and remaining blockers.
- When changing contracts, mention compatibility and consumer updates.
