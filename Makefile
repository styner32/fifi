.DEFAULT_GOAL := help
# Keep `make -C /path mcp` free of directory banners on stdout as well.
MAKEFLAGS += --no-print-directory

GO ?= go
NPM ?= npm
DOCKER_COMPOSE ?= docker compose
MIGRATE ?= $(GO) run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1
CMD_DIR ?= cmd
MIGRATIONS_DIR ?= internal/db/migrations
DATE ?= $(shell TZ=Asia/Seoul date +%Y%m%d)
ARGS ?=
SYMBOLS ?=
WATCHLIST ?=
RECEIPT_NO ?=
CORP_CODE ?=
LIMIT ?=
SERVICE ?=
TEST_PACKAGES ?= ./cmd/... ./internal/...

# Resolve environment > .env > local defaults in one place. Never echo DSNs.
ENV_RUN = $(GO) run ./cmd/local-env
TEST_ENV_RUN = $(ENV_RUN) --test

.PHONY: help setup deps submodule-init submodule-update services-up services-down services-status services-logs redis-prepare \
	run agent premarket now now-debug takesnapshot filings-snapshot creditbalance leverage rsi safety-devices \
	dart-filing-cli dart-filing-cli-companies dart-filing-cli-company dart-filing-cli-reports dart-filing-api dart-filing-worker \
	market-collector market-collector-once market-collector-backfill market-collector-filings market-collector-filings-once \
	web-install web-build web-lint dart-filing-web dart-filing-web-build \
	build app-build agent-build market-collector-build dart-filing-api-build dart-filing-worker-build dart-filing-cli-build \
	mcp mcp-build market-mcp market-mcp-build dart-filing-mcp dart-filing-mcp-build \
	test test-integration test-race test-db-prepare check check-docs audit audit-go audit-web \
	migrate-up migrate-down migrate-status migrate-create migrate-test-up migrate-test-redo

help: ## Show local setup, execution and verification commands (default)
	@ruby -ne 'if /^([a-zA-Z0-9_-]+):.*?## (.*)/; printf "  %-31s %s\n", $$1, $$2; end' Makefile

setup: ## Install locked dependencies and initialize the pinned reference submodule
	@$(MAKE) --no-print-directory submodule-init
	@$(MAKE) --no-print-directory deps

deps: ## Download Go modules and install the npm lockfile
	$(GO) mod download
	@$(MAKE) --no-print-directory web-install

submodule-init: ## Initialize the reference submodule at its recorded commit
	git submodule update --init --recursive

submodule-update: ## Explicitly update the reference submodule to latest upstream
	git submodule update --remote --merge external/open-trading-api

services-up: ## Start local PostgreSQL/Redis, preserving existing data
	@$(MAKE) --no-print-directory redis-prepare
	$(DOCKER_COMPOSE) up -d --wait db redis

redis-prepare: ## Prepare the local Redis 8 copy once; keep the original volume
	@DOCKER_COMPOSE="$(DOCKER_COMPOSE)" ruby scripts/prepare_local_redis.rb

services-down: ## Stop local containers without removing data volumes
	$(DOCKER_COMPOSE) stop

services-status: ## Show local container status
	$(DOCKER_COMPOSE) ps -a

services-logs: ## Follow local container logs [SERVICE=db|redis|...]
	$(DOCKER_COMPOSE) logs --follow --tail=100 $(SERVICE) $(ARGS)

run: ## Run the integrated stock/futures/company example
	@$(ENV_RUN) $(GO) run ./$(CMD_DIR) $(ARGS)

agent: ## Run any report: make agent ARGS='report rsi --symbol kospi'
	@$(ENV_RUN) $(GO) run ./cmd/agent $(ARGS)

premarket: ## Collect and save the premarket report [ARGS='--json --no-save']
	@$(ENV_RUN) $(GO) run ./cmd/agent report premarket $(ARGS)

now: ## Collect and save the intraday pulse [ARGS='--json --no-save']
	@$(ENV_RUN) $(GO) run ./cmd/agent report intraday-pulse $(ARGS)

now-debug: ## Collect a pulse without report/history writes
	@$(ENV_RUN) $(GO) run ./cmd/agent report intraday-pulse $(ARGS) --no-history --no-save

takesnapshot: ## Save a daily snapshot [DATE=YYYYMMDD] (default: KST today)
	@$(ENV_RUN) $(GO) run ./cmd/agent report market-snapshot --date "$(DATE)" $(ARGS)

filings-snapshot: ## Save only the filings snapshot [DATE=YYYYMMDD]
	@$(ENV_RUN) $(GO) run ./cmd/agent report filings-snapshot --date "$(DATE)" $(ARGS)

creditbalance: ## Credit report [SYMBOLS='005930 ...' or WATCHLIST=path]; default: market ratio
	@if [ -n "$(SYMBOLS)" ]; then \
		$(ENV_RUN) $(GO) run ./cmd/agent report credit-balance $(SYMBOLS) $(ARGS); \
	elif [ -n "$(WATCHLIST)" ]; then \
		$(ENV_RUN) $(GO) run ./cmd/agent report credit-balance --watchlist "$(WATCHLIST)" $(ARGS); \
	else \
		$(ENV_RUN) $(GO) run ./cmd/agent report credit-balance --ratio $(ARGS); \
	fi

leverage: ## Show the market credit/deposit ratio [ARGS='--days 60 --json']
	@$(ENV_RUN) $(GO) run ./cmd/agent report credit-balance --ratio $(ARGS)

rsi: ## Show RSI [ARGS='--symbol kospi --period 14 --interval 60m']
	@$(ENV_RUN) $(GO) run ./cmd/agent report rsi $(ARGS)

safety-devices: ## Show circuit-breaker/sidecar observations
	@$(ENV_RUN) $(GO) run ./cmd/agent report safety-devices $(ARGS)

dart-filing-cli: ## Analyze one filing without DB writes [RECEIPT_NO=...]
	@test -n "$(RECEIPT_NO)" || { echo 'Usage: make dart-filing-cli RECEIPT_NO=<receipt_number>' >&2; exit 1; }
	@$(ENV_RUN) $(GO) run ./cmd/dart-filing-worker-cli dry-run "$(RECEIPT_NO)" $(ARGS)

dart-filing-cli-companies: ## Backfill the DART company master
	@$(ENV_RUN) $(GO) run ./cmd/dart-filing-worker-cli companies $(ARGS)

dart-filing-cli-company: ## Collect/analyze company filings [CORP_CODE=... LIMIT=5]
	@test -n "$(CORP_CODE)" || { echo 'Usage: make dart-filing-cli-company CORP_CODE=<corp_code> [LIMIT=5]' >&2; exit 1; }
	@$(ENV_RUN) $(GO) run ./cmd/dart-filing-worker-cli company "$(CORP_CODE)" $(LIMIT) $(ARGS)

dart-filing-cli-reports: ## Explicitly collect/analyze recent DART filings
	@$(ENV_RUN) $(GO) run ./cmd/dart-filing-worker-cli reports $(ARGS)

dart-filing-api: ## Run the DART REST API locally
	@$(ENV_RUN) $(GO) run ./cmd/dart-filing-api $(ARGS)

dart-filing-worker: ## Run the DART worker locally (existing queues are preserved)
	@$(ENV_RUN) $(GO) run ./cmd/dart-filing-worker $(ARGS)

market-collector: ## Run the market facts collector daemon
	@$(ENV_RUN) $(GO) run ./cmd/market-collector run $(ARGS)

market-collector-once: ## Collect market facts once [ARGS='--force=false']
	@$(ENV_RUN) $(GO) run ./cmd/market-collector once $(ARGS)

market-collector-backfill: ## Backfill verified facts from saved reports
	@$(ENV_RUN) $(GO) run ./cmd/market-collector backfill $(ARGS)

market-collector-filings: ## Run the major-company filings collector daemon
	@$(ENV_RUN) $(GO) run ./cmd/market-collector filings run $(ARGS)

market-collector-filings-once: ## Collect major-company filings once [ARGS='--dry-run']
	@$(ENV_RUN) $(GO) run ./cmd/market-collector filings once $(ARGS)

web-install: ## Install frontend dependencies with npm ci
	$(NPM) --prefix web ci

dart-filing-web: ## Run the Vite dev server [ARGS='--host 127.0.0.1']
	$(NPM) --prefix web run dev -- $(ARGS)

dart-filing-web-build: ## Build the frontend
	$(NPM) --prefix web run build

web-build: dart-filing-web-build

web-lint: ## Lint the frontend
	$(NPM) --prefix web run lint

build: app-build agent-build market-collector-build dart-filing-api-build dart-filing-worker-build dart-filing-cli-build mcp-build ## Build all local application/CLI/MCP binaries

app-build:
	$(GO) build -trimpath -o bin/kis-open-api ./$(CMD_DIR)

agent-build:
	$(GO) build -trimpath -o bin/agent ./cmd/agent

market-collector-build:
	$(GO) build -trimpath -o bin/market-collector ./cmd/market-collector

dart-filing-api-build:
	$(GO) build -trimpath -o bin/dart-filing-api ./cmd/dart-filing-api

dart-filing-worker-build:
	$(GO) build -trimpath -o bin/dart-filing-worker ./cmd/dart-filing-worker

dart-filing-cli-build:
	$(GO) build -trimpath -o bin/dart-filing-worker-cli ./cmd/dart-filing-worker-cli

mcp: ## Start the unified market/disclosure stdio MCP (six tools)
	@$(ENV_RUN) $(GO) run ./cmd/fifi-mcp $(ARGS)

mcp-build: ## Build bin/fifi-mcp and refresh compatible executable names
	$(GO) build -trimpath -o bin/fifi-mcp ./cmd/fifi-mcp
	$(GO) build -trimpath -o bin/market-mcp ./cmd/market-mcp
	$(GO) build -trimpath -o bin/dart-filing-mcp ./cmd/dart-filing-mcp

market-mcp dart-filing-mcp: mcp ## Compatibility aliases for the unified MCP; run only one

market-mcp-build dart-filing-mcp-build: mcp-build

test: test-integration ## Run all Go suites against the isolated *_test database

test-integration: ## Run uncached Go suites; reject a non-test database
	@$(TEST_ENV_RUN) $(GO) test -p 1 -count=1 $(TEST_PACKAGES) $(ARGS)

test-race: ## Run Go suites with race detection on the isolated database
	@$(TEST_ENV_RUN) $(GO) test -p 1 -race -count=1 $(TEST_PACKAGES) $(ARGS)

test-db-prepare: ## Create the local *_test database if absent, then migrate it
	@$(TEST_ENV_RUN) env DOCKER_COMPOSE="$(DOCKER_COMPOSE)" ruby scripts/prepare_test_database.rb
	@$(MAKE) --no-print-directory migrate-test-up

check: test-integration check-docs web-build web-lint ## Verify Go suites, docs, frontend build and lint

check-docs: ## Check active documentation links
	ruby scripts/check_doc_links.rb

audit: audit-go audit-web ## Check Go call paths and the complete npm lockfile for vulnerabilities

audit-go:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./cmd/... ./internal/...

audit-web:
	$(NPM) --prefix web audit

# The shell expands the launcher's resolved environment; make never prints a URL.
migrate-up: ## Apply all local database migrations
	@$(ENV_RUN) sh -c 'exec "$$@" -database "$$DATABASE_URL" up' sh $(MIGRATE) -path "$(MIGRATIONS_DIR)"

migrate-down: ## Roll back one local migration (explicitly requested only)
	@$(ENV_RUN) sh -c 'exec "$$@" -database "$$DATABASE_URL" down 1' sh $(MIGRATE) -path "$(MIGRATIONS_DIR)"

migrate-status: ## Show the local database migration version
	@$(ENV_RUN) sh -c 'exec "$$@" -database "$$DATABASE_URL" version' sh $(MIGRATE) -path "$(MIGRATIONS_DIR)"

migrate-create: ## Create migration files [NAME=description]
	@test -n "$(NAME)" || { echo 'Usage: make migrate-create NAME=<description>' >&2; exit 1; }
	$(MIGRATE) create -ext sql -dir "$(MIGRATIONS_DIR)" -seq "$(NAME)"

migrate-test-up: ## Apply migrations to the guarded *_test database
	@$(TEST_ENV_RUN) sh -c 'exec "$$@" -database "$$DATABASE_URL" up' sh $(MIGRATE) -path "$(MIGRATIONS_DIR)"

migrate-test-redo: ## Roll back/reapply one migration on the guarded *_test database
	@$(TEST_ENV_RUN) sh -c 'exec "$$@" -database "$$DATABASE_URL" down 1' sh $(MIGRATE) -path "$(MIGRATIONS_DIR)"
	@$(MAKE) --no-print-directory migrate-test-up
