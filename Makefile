BACKEND_DIR := backend
GOOSE_CMD ?= goose
MIGRATIONS_DIR := $(BACKEND_DIR)/migrations

-include .env
export

GOOSE_DRIVER ?= postgres
GOOSE_DBSTRING ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: run migrate migrate-down test lint

run:
	cd $(BACKEND_DIR) && ENV_FILE=../.env go run ./cmd/server

migrate:
	$(GOOSE_CMD) -dir $(MIGRATIONS_DIR) up

migrate-down:
	$(GOOSE_CMD) -dir $(MIGRATIONS_DIR) down

test:
	cd $(BACKEND_DIR) && go test ./...

lint:
	cd $(BACKEND_DIR) && golangci-lint run ./...
