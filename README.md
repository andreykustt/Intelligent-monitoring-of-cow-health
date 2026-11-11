# Cow Health Monitor — AI agent starter kit

This is a **documentation and instruction starter kit**, not a completed application. It is designed for coding agents (Codex/Claude Code and similar) to implement a cattle-health monitoring MVP without loading all docs on each task.

## Quick start
1. Extract these files as the root of a new Git repository.
2. Read `docs/architecture.md`, `docs/implementation-plan.md` and `docs/acceptance.md` once as project owner.
3. Give your agent `prompts/first-task.md` as its first bounded task; then use `prompts/task-template.md` for later tasks.
4. Keep `AGENTS.md` stable during active sessions; add implementation results to `docs/progress.md`.

## Design decisions
- Only admin and operator roles.
- Real-time automatic sensor metrics plus manual milking, lab and inspection results.
- React application includes dashboards; no Grafana.
- Go services, Python ML, PostgreSQL, RabbitMQ, Docker Compose.
- All hardware readings in MVP are simulated or replayed. ML detects anomalies, not confirmed diseases.

## Where to look
`AGENTS.md` = minimal global rules; local `AGENTS.md` = module-specific; `.claude/rules` = conditional Claude Code hints; `docs/` = source of truth for architecture/API/data/ML/acceptance; `prompts/` = reusable scoped work instructions.

No source code, Compose implementation, test results or datasets are included yet. The commands referenced in instructions become executable as the implementation is added.
