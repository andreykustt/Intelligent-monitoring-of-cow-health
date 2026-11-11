# React frontend

- React + TypeScript + Vite; TanStack Query for REST, TanStack Table for tables, ECharts for charts.
- Only `admin` and `operator`; UI gating supplements (never replaces) server-side RBAC.
- REST for initial/historical data, WebSocket for updates. On reconnect/refocus, refetch snapshot to repair gaps.
- Do not render every broker message: batch/throttle stream UI, cap retained rows and virtualize long lists.
- Sorting, filtering, pagination over full datasets happen on the server; do not pretend loaded page equals all data.
- Manual forms: cow ID, measurement time, units, required fields, source, operator identity; display correction history.
- Keep 'no anomaly' distinct from 'insufficient data' and stale/missing sensor readings.
- Do not present synthetic cases as real diagnoses or claim unsupported ML explanations.
- API types originate from `docs/api.md` / generated OpenAPI definitions. Consult `docs/data-contracts.md` for units.
