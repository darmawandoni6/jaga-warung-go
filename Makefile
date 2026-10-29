.PHONY: help run build test vet fmt docker docker-down docker-logs gate

# Defaults
PORT      ?= 8080
DATABASE  ?= host=127.0.0.1 user=postgres password=postgres dbname=jaga_warung port=5432 sslmode=disable TimeZone=Asia/Jakarta
JWT       ?= dev-only
PHONE     ?= 081234567890
PASSWORD  ?= devsecret123
COMPOSE   ?= podman-compose

help:
	@echo 'Targets:'
	@echo '  make run          — local dev (requires running Postgres)'
	@echo '  make docker       — start db + api with podman-compose'
	@echo '  make docker-db    — start only postgres in background'
	@echo '  make docker-down  — stop compose stack'
	@echo '  make docker-logs  — follow api logs'
	@echo '  make build        — go build ./...'
	@echo '  make test         — go test ./...'
	@echo '  make vet          — go vet ./...'
	@echo '  make fmt          — gofmt -w . && go vet ./...'
	@echo '  make gate         — fmt + vet + build + test'
	@echo ''
	@echo 'Env overrides:'
	@echo '  PORT=8080      api listen port'
	@echo '  DATABASE=...   postgres DSN (default: 127.0.0.1:5432/jaga_warung)'
	@echo '  JWT=...        jwt secret'
	@echo '  PHONE=...      seeded admin phone'
	@echo '  PASSWORD=...   seeded admin password'

run:
	JWT_SECRET="$(JWT)" \
	DATABASE_URL="$(DATABASE)" \
	PORT="$(PORT)" \
	LOG_LEVEL=debug \
	ADMIN_PHONE="$(PHONE)" \
	ADMIN_PASSWORD="$(PASSWORD)" \
	go run ./cmd/api

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .
	@go vet ./...

docker:
	$(COMPOSE) up --build

docker-db:
	$(COMPOSE) up -d db

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f api

gate: fmt vet build test
