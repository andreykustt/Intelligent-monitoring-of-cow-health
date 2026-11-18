.PHONY: up down check test smoke api-types
up:
	docker compose up -d --build

down:
	docker compose down

check:
	docker compose config -q
	cd services && go vet ./...
	cd frontend && npm run lint && npm run build

test:
	cd services && go test ./...
	python3 scripts/smoke.py

smoke:
	python3 scripts/smoke.py

api-types:
	python3 scripts/generate_openapi.py
	cd frontend && npm run generate:api
