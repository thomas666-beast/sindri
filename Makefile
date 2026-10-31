SHELL := /bin/bash
.DEFAULT_GOAL := help

-include .env
export DATABASE_URL
export DATABASE_URL_TEST

.PHONY: help setup run test test-verbose migrate migrate-create migrate-down \
        sqlc build lint fmt tidy clean worker promote web-dev seed seed-clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "};{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

setup: ## One-time: install tools, copy .env, verify DB reachable
	@test -f .env || cp .env.example .env
	@command -v goose >/dev/null || go install github.com/pressly/goose/v3/cmd/goose@latest
	@command -v sqlc  >/dev/null || go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@pg_isready -h localhost -p 5432 >/dev/null || (echo "✗ postgres not reachable on localhost:5432 — is it running?"; exit 1)
	@psql "$(DATABASE_URL)" -c "SELECT 1" >/dev/null || (echo "✗ cannot connect with DATABASE_URL — check .env"; exit 1)
	@psql "$(DATABASE_URL_TEST)" -c "SELECT 1" >/dev/null || (echo "✗ cannot connect with DATABASE_URL_TEST — check .env"; exit 1)
	@echo "✓ setup complete — run 'make run'"

run: ## Run the API server
	go run ./cmd/api

test: ## Run all tests (against DATABASE_URL_TEST)
	DATABASE_URL=$(DATABASE_URL_TEST) go test ./... -count=1

test-verbose: ## Run tests with -v and -race (against DATABASE_URL_TEST)
	DATABASE_URL=$(DATABASE_URL_TEST) go test ./... -count=1 -v -race

migrate: ## Apply pending migrations
	goose -dir migrations postgres "$(DATABASE_URL)" up
	goose -dir migrations postgres "$(DATABASE_URL_TEST)" up

migrate-create: ## Create migration: make migrate-create NAME=create_users
	@test -n "$(NAME)" || (echo "usage: make migrate-create NAME=<name>"; exit 1)
	goose -dir migrations postgres "$(DATABASE_URL)" create $(NAME) sql

migrate-down: ## Roll back the last migration
	goose -dir migrations postgres "$(DATABASE_URL)" down
	goose -dir migrations postgres "$(DATABASE_URL_TEST)" down

sqlc: ## Generate type-safe Go from SQL
	sqlc generate

build: ## Build project
	go build ./...

lint: ## go vet
	go vet ./...

fmt: ## Format Go code
	gofmt -w .

tidy: ## Tidy go.mod
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin/ dist/ coverage.out coverage.html

worker: ## Run the auto-release worker
	go run ./cmd/worker

promote: ## Promote a user to admin: make promote EMAIL=foo@bar.com
	@test -n "$(EMAIL)" || (echo "usage: make promote EMAIL=<email>"; exit 1)
	go run ./cmd/admin-cli "$(EMAIL)"

web-dev: ## Run frontend dev server
	cd web && npm run dev

seed: ## Seed 1000 users + products + wallets (uses DATABASE_URL)
	go run ./cmd/seed

seed-clean: ## Remove seeded sindri_* users and their data
	go run ./cmd/seed -clean

seed-named: ## Re-create only the three named accounts (pitrucci, romeo, james)
	go run ./cmd/seed -users 0
