# Comandos de SWEVEH (docs/05 §8). En Windows, córrelos desde Git Bash.
# Las herramientas se fijan por versión aquí; cambiar una versión es un cambio revisable.

SQLC_IMAGE      := sqlc/sqlc:1.29.0
GOLANGCI        := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
GOVULNCHECK     := golang.org/x/vuln/cmd/govulncheck@v1.9.0
GITLEAKS_IMAGE  := zricethezav/gitleaks:v8.30.1

-include .env
export

.PHONY: help up down dev dev-api dev-worker dev-web agent-eval migrate migrate-down gen gen-check lint test test-go test-web cover vuln secrets ci build image e2e seed-demo reset-demo

help: ## Lista los comandos
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

up: ## Levanta Postgres local (docker compose)
	docker compose up -d --wait postgres

down: ## Apaga Postgres local (conserva los datos)
	docker compose down

migrate: up ## Aplica las migraciones a la base local con sweveh_migrator
	go run ./cmd/migrate up

migrate-down: ## Revierte la última migración (solo local)
	go run ./cmd/migrate down

dev: up migrate ## Instrucciones para levantar api, worker y web en tres terminales
	@echo "Abre tres terminales:"
	@echo "  make dev-api      # api en :8080 (el código de entrada sale en esta terminal)"
	@echo "  make dev-worker   # despacha el outbox"
	@echo "  make dev-web      # Vite en http://localhost:5173 (reenvía /api al api)"

dev-api: ## Corre el api en local
	go run ./cmd/api

dev-worker: ## Corre el worker en local
	go run ./cmd/worker

dev-web: ## Corre el frontend con Vite
	npm --prefix web run dev

agent-eval: ## Mide el intérprete configurado (AI_PROVIDER) con los 30 mensajes de HU-102
	go run ./cmd/agent-eval

gen: ## Regenera sqlc y los tipos TypeScript del contrato OpenAPI
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) generate
	npm --prefix web run gen:api

gen-check: gen ## Falla si el código generado no está commiteado
	git diff --exit-code -- internal/*/adapters/out/postgres/sqlcgen web/src/shared/api/schema.d.ts

lint: ## golangci-lint (con depguard), ESLint y tsc
	go run $(GOLANGCI) run ./...
	npm --prefix web run lint
	npm --prefix web run typecheck

test: test-go test-web ## Todas las pruebas

test-go: up ## go test -race con Postgres real (incluye aislamiento y concurrencia)
	go test -race -count=1 ./...

test-web: ## Vitest
	npm --prefix web test -- --run

cover: up ## Cobertura de Go; falla si dominio o aplicación quedan bajo 80 % (R-11)
	sh scripts/coverage.sh

vuln: ## govulncheck
	go run $(GOVULNCHECK) ./...

secrets: ## gitleaks sobre el árbol de trabajo
	docker run --rm -v "$(CURDIR):/repo" $(GITLEAKS_IMAGE) dir /repo --no-banner --redact --config /repo/.gitleaks.toml

ci: gen-check lint cover test-web vuln secrets build ## Lo mismo que el pipeline
	@echo "make ci: todo en verde"

build: ## Compila los binarios y el frontend
	go build -o bin/ ./cmd/...
	npm --prefix web run build

image: ## Construye la imagen de contenedor
	docker build -f deploy/Dockerfile -t sweveh:local .

e2e: migrate ## Playwright: la orden de punta a punta con el api sirviendo la SPA (scripts/e2e.sh)
	sh scripts/e2e.sh

seed-demo: ## Taller ficticio (pendiente: HU-97/HU-98)
	@echo "make seed-demo todavía no existe: llega con HU-97 y HU-98 (T5)." >&2; exit 1

reset-demo: ## Reinicia el taller ficticio (pendiente: HU-97)
	@echo "make reset-demo todavía no existe: llega con HU-97 (T5)." >&2; exit 1
