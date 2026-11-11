# Deployment rules

- Compose-only MVP; single-host deployment. No Kubernetes or infra monitoring stack.
- Nginx exposes frontend, `/api/` and WebSocket upgrade; DB, RabbitMQ and ML endpoints are private.
- Keep persistent named volumes for Postgres/RabbitMQ; `docker compose down -v` is destructive and must not be default advice.
- Add healthchecks and app-level retry because `depends_on` alone is not application readiness.
- Credentials via env or secrets; commit `.env.example`, never `.env`.
- CI should validate/test/build individual components before publishing images; pin image versions in deploy configs.
- Production-like deployments must secure TLS, WebSocket origins, and RabbitMQ credentials.
