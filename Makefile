# Source .env when present — imported as make variables so run: picks them up.
# Command-line overrides still win; ?= defaults only apply when .env is absent.
-include .env

.PHONY: help run worker build test vet fmt docker docker-db docker-kafka docker-down docker-logs worker-logs gate

# Defaults — applied only when .env is absent or doesn't set the key.
PORT            ?= 8080
DATABASE_URL    ?= host=127.0.0.1 user=postgres password=postgres dbname=jaga_warung port=5432 sslmode=disable TimeZone=Asia/Jakarta
JWT_SECRET      ?= dev-only
ADMIN_PHONE     ?= 081234567890
ADMIN_PASSWORD  ?= devsecret123
LOG_LEVEL       ?= info
COMPOSE         ?= podman-compose
# Empty = Kafka publisher disabled (no events, no broker needed).
KAFKA_BROKERS   ?=
KAFKA_GROUP_ID  ?= jaga-warung-worker

help:
	@echo 'Targets:'
	@echo '  make run          — local dev (requires running Postgres; reads .env when present)'
	@echo '  make worker       — kafka log-only consumer (needs KAFKA_BROKERS)'
	@echo '  make docker       — start db + kafka + api + worker with podman-compose (reads .env via docker-compose)'
	@echo '  make docker-db    — start only postgres in background'
	@echo '  make docker-kafka — start only kafka + topic init in background (optional, publisher needs KAFKA_BROKERS)'
	@echo '  make docker-down  — stop compose stack'
	@echo '  make docker-logs  — follow api logs'
	@echo '  make worker-logs  — follow worker logs'
	@echo '  make build        — go build ./...'
	@echo '  make test         — go test ./...'
	@echo '  make vet          — go vet ./...'
	@echo '  make fmt          — gofmt -w . && go vet ./...'
	@echo '  make gate         — fmt + vet + build + test'
	@echo ''
	@echo 'Env overrides (highest precedence):'
	@echo '  PORT=8080           api listen port'
	@echo '  DATABASE_URL=...    postgres DSN'
	@echo '  JWT_SECRET=...      jwt secret'
	@echo '  ADMIN_PHONE=...     seeded admin phone'
	@echo '  ADMIN_PASSWORD=...  seeded admin password'
	@echo '  LOG_LEVEL=...       log level'
	@echo '  KAFKA_BROKERS=...   comma-separated brokers; empty = publisher off'
	@echo '  KAFKA_GROUP_ID=...  consumer group for make worker (default jaga-warung-worker)'
	@echo ''
	@echo 'Dotenv: variables from .env are imported into make and used as defaults.'
	@echo 'Command-line > .env > Makefile ?= defaults.'

run:
	JWT_SECRET="$(JWT_SECRET)" \
	DATABASE_URL="$(DATABASE_URL)" \
	PORT="$(PORT)" \
	LOG_LEVEL="$(LOG_LEVEL)" \
	ADMIN_PHONE="$(ADMIN_PHONE)" \
	ADMIN_PASSWORD="$(ADMIN_PASSWORD)" \
	KAFKA_BROKERS="$(KAFKA_BROKERS)" \
	go run ./cmd/api

worker:
	KAFKA_BROKERS="$(KAFKA_BROKERS)" \
	KAFKA_GROUP_ID="$(KAFKA_GROUP_ID)" \
	LOG_LEVEL="$(LOG_LEVEL)" \
	go run ./cmd/worker

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

docker-kafka:
	$(COMPOSE) up -d kafka kafka-init

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f api

worker-logs:
	$(COMPOSE) logs -f worker

gate: fmt vet build test
