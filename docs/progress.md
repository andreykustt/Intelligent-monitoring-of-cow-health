# Progress log

## P0-05, P1-08 и P2-04

- Базовая smoke-проверка запускает отдельный Compose-проект и ждёт фактической остановки симуляции.
- OpenAPI и TS-типы описывают необязательный комментарий при подтверждении уведомления и ответ `202` с версией при изменении конфигурации симуляции. Поведение Go API прежнее.
- Нагрузочный сценарий на 200 животных считает записи конкретной симуляции и измеряет HTTP-ответы после прогрева. Новые результаты сохраняются локально.
- Локально проверены сценарии ожидания остановки, очистки стенда при ошибке и расчёта отчёта с подменой зависимостей. В CI проверяются Compose и регенерация TS-типов. Отдельное задание запускает полный замер при изменении нагрузочного скрипта или запускающего его стенда.

## 2026-11-18 — First executable educational MVP

Implemented from a documentation-only repository. Implementation and documentation are grouped into local commits; no push was performed. See the commit-grouping entry below.

### Delivered

- Compose, Nginx, private PostgreSQL/RabbitMQ, persistent volumes, healthchecks, explicit migration runner and idempotent seed (200 demo cows; credentials from ignored .env).
- Five Go service binaries, migrate and seed in a shared Go module. Thin cmd entrypoints; internal packages preserve documented ownership.
- Session login/logout/me, bcrypt, hashed session tokens, CSRF, admin/operator permissions and per-farm checks including WebSocket.
- Animal CRUD, server search/filter/sort/pagination, latest readings, bounded aggregated history, overview and manual milk-yield aggregation.
- Confirmed RabbitMQ ingestion; transactional idempotent storage/latest-state ordering; owner-scoped transactional outbox; bounded best-effort WS with reconnect snapshot refresh.
- Synthetic/replay simulator, per-metric frequency, deterministic seed, anomaly/missing scenarios, versioned durable control commands, actual status events, pause/resume/stop/config.
- Five typed/validated manual payload kinds with common JSONB persistence, idempotency keys, optimistic locking, append-only revisions and actor/time audit.
- Offline synthetic Isolation Forest pipeline, FastAPI inference and asynchronous Go analysis jobs; insufficient_data distinct from no_anomaly/anomaly; active-alert dedupe and action history.
- Russian React interface with own responsive design, TanStack Query/Table and ECharts. Login, overview, monitor, profile/charts, manual forms/revisions, alerts, simulation/animal/user administration.
- OpenAPI source generator + generated contract + generated TypeScript. CI for Go/frontend/Compose/HTTP acceptance. Operational guide and local load report.

### Verified

- `docker compose config -q`: passed.
- Container builds for all components: passed; stack starts with explicit migrations/seed. Nginx healthcheck corrected to IPv4 loopback. Services healthy after fault recovery.
- Go `go test ./...`, `go vet ./...`, gofmt: passed (validation, permissions, units, BCS, pagination, simulation config cases).
- ML image: `python -m pytest -q` — 4 passed; `python -m ruff check .` passed; deterministic offline training passed.
- Frontend clean npm installation in Node/Alpine container; generated types, ESLint, TypeScript and Vite production build passed. Final npm audit: 0 vulnerabilities. Charts use modular imports/separate chunk.
- Curl acceptance: session/anonymous access, operator restrictions, CSRF, server query validation, cows/metrics/history/overview, all manual kinds, idempotency, correction/version conflicts/history, simulator start/pause/resume/stop, persisted readings, logout revocation.
- Curl WS: foreign-farm rejection, HTTP 101 upgrade and actual committed-event frames.
- Delivery acceptance: duplicate IDs produce one row; old events remain historical without replacing latest; telemetry survives ML outage; broker outage returns 503; services restored.
- Synthetic historical replay produces anomaly prediction and notification; detail/acknowledge/resolve/action history verified through HTTP.
- User administration: last-admin guard, creation, blocking/session revocation, password reset, cross-farm card/history/WS isolation and forbidden farm assignment passed.
- Controlled 200-cow/9-metric run: about 357 stored events/s, list wall p95 about 71ms including curl process overhead. See load-report.json; not a capacity guarantee.

### Compatibility / deliberate implementation choices

First v1 implementation: no deployed consumers existed. api.md/data-contracts.md/database.md/ml.md describe DTOs, added routes, physical JSONB manual-record layout and model window_start addition. Frontend consumes generated types. Go services share one module instead of duplicating dependency modules. The durable simulation command table is reconciled by simulator; broker command events are integration notifications.

### Remaining limits

- Browser installation/testing was explicitly excluded by the user. Playwright scenarios are supplied but were not run; visual rendering/accessibility and browser reconnect interaction remain unverified. Default CI acceptance uses HTTP, not browser installation.
- Real licensed datasets were not researched/integrated in this implementation. Model is a synthetic feature-level educational baseline; no clinical validation or disease prediction.
- Finalized analysis windows are immutable; late historical backfill does not silently rewrite them. A 10s arrival settling delay reduces partial-window races.
- Simulator distribution ranges are code-configured; UI adjusts metrics/frequencies/scenario, not arbitrary distributions. Replay is inline and bounded to 1000 events. Restart of an ambiguous active simulator marks it failed rather than claiming lossless continuation.
- Farm grouping/selection exists; arbitrary cow-group management and extended analytical reports are deferred.
- 1000-cow extended load, browser render latency, quantitative WS lag, migration rollback/backup-restore drills and production TLS deployment were not verified. No performance guarantee outside the measured local baseline.
- This delivers the primary end-to-end MVP flows across M1–M6; it does not mark every original M7 acceptance/load item complete.

### Next bounded task

See [the improvement backlog](improvement-plan.md). Start with isolated regression checks (P0-05), then auth/error handling (P0-01/P0-02) and delivery/routing fixes (P0-03/P0-04). Extended load/restore drills and dataset research follow the correctness work. Browser QA must not install a browser without a new user request.

## 2026-11-18 — Русский README и правила Git

- Обновлён `README.md`: описание проекта, возможности и роли, архитектура, требования, первый и повторный запуск, демонстрация, переменные окружения, диагностика, проверки без браузера, локальная разработка и ограничения MVP.
- Расширен `.gitignore`: секреты и локальные настройки, зависимости и кэши, сборки и тестовые отчёты, модели и локальные датасеты, резервные копии, файлы редакторов и ОС. Шаблоны `.env.example`, исходники, миграции, документация и lock-файлы остаются доступными для Git.
- Проверено через `git check-ignore`: 19 исключаемых и 13 сохраняемых путей; все проверки прошли. Все 13 локальных ссылок README существуют; синтаксис 9 shell-примеров проверен через `sh -n`.
- `git diff --check` и `docker compose config -q`: успешно. Сборка и тесты приложения повторно не запускались: изменены документация и правила Git, исполняемый код и контракты не менялись. Блокеров нет.

## 2026-11-18 — План доработки после аудита

- Создан `docs/improvement-plan.md`: 18 открытых задач P0–P2, обнаруженные проблемы, работы, зависимости, критерии готовности и порядок выполнения.
- Зафиксировано согласованное целевое поведение паузы: 10-минутный запуск после паузы на второй минуте должен доработать оставшиеся 8 минут; replay во время паузы не продвигается. Это задача, а не уже выполненное изменение.
- В README добавлены ближайшие приоритеты, текущие проблемы и ссылка на подробный план. Обновлена следующая ограниченная задача в журнале. Исполняемый код, контракты и зависимости не менялись.
- Проверено: 26 локальных ссылок в трёх документах существуют; 18 уникальных открытых задач соответствуют 18 подробным описаниям; согласованное поведение паузы и раздел приоритетов README присутствуют. `git diff --check` прошёл. Тесты приложения повторно не запускались: изменена только документация. Блокеров нет.

## 2026-11-18 — Изменения разделены на логические коммиты

- Использована существующая Git-идентичность пользователя: `Andrey Kust <a.g.kust@yandex.ru>` для автора и коммитера; дополнительные соавторы не добавлялись.
- Изменения сгруппированы в девять частей: настройки репозитория; общие Go-компоненты и БД; телеметрия и симулятор; API и контракты; ML; React-интерфейс; Docker-развёртывание; интеграционные проверки и CI; документация, UX-аудит и план доработки.
- Перед коммитами проверены кандидаты на добавление: значения пяти секретных переменных из локального `.env` и приватные ключи не обнаружены, файлов больше 2 МБ нет. `.env`, зависимости, сборки, кэши и модели исключены правилами Git.
- Убрана лишняя пустая строка в конце `scripts/http_client.py`, найденная `git diff --cached --check`; поведение кода не менялось. Проверки приложения повторно не запускались. Коммиты созданы локально, отправка в удалённый репозиторий не выполнялась.
