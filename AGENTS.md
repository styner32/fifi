# Project Context: fifi (Financial & Trading Platform)

This file documents the project's technology stack, architectural decisions, development preferences, and domain rules for AI assistants.

## Tech Stack & Architecture

### Backend
- **Language:** Go (v1.27.2, pinned in `go.mod` and `.go-version`)
- **Web Framework:** Gin
- **Database:** PostgreSQL (GORM ORM, `golang-migrate`)
- **Job Queue:** Asynq (Redis-backed)
- **Testing:** Ginkgo & Gomega
- **AI/LLM:** OpenAI API (`openai-go`)
- **HTML/XBRL Parsing:** goquery

### Frontend (`web/`)
- **Language:** TypeScript
- **Framework:** React SPA (Vite)
- **Toolchain:** Node 24.22.0 LTS / npm 12.2.0; TypeScript 6.0.3 (the supported typescript-eslint range excludes TypeScript 7)
- **Dev Server:** `npm --prefix web run dev` (`make dart-filing-web`)
- **Build Tool:** `npm --prefix web run build` (`make dart-filing-web-build`)

### Submodules (`external/`)
- **open-trading-api:** `external/open-trading-api` (Submodule tracking `https://github.com/koreainvestment/open-trading-api.git`)

### Infrastructure & Tools
- **Containerization:** Docker Compose (PostgreSQL 18.6, Redis 8.10.2; preserve the original Redis Stack volume during migration)
- **Migrations:** `golang-migrate` (`make migrate-up`)
- **Task Runner:** Makefile

## Development Preferences

### Scripting & Ad-hoc Tasks
- **Language:** Ruby / Go CLI
- **Use Case:** Use Go CLI tools (`cmd/agent`, `cmd/dart-filing-worker-cli`) or Ruby for utility scripts and data automation.

### Testing
- **Required Go test style:** Write all new or modified unit, integration, and regression test cases with **Ginkgo v2 + Gomega in BDD style**.
- Organize behavior with `Describe` → `Context` → `It`. Use `DescribeTable` / `Entry` for repeated cases and Gomega matchers for assertions.
- Reuse the package's existing `*_suite_test.go`. The standard `testing` package and `TestXxx(t *testing.T)` are used for the Ginkgo suite entry point (`RegisterFailHandler` / `RunSpecs`), not standalone behavior tests. Do not add `t.Run` / `t.Fatal` based test cases.
- Keep each spec independent with `BeforeEach`, fixed inputs/clocks, and `GinkgoT().TempDir()` where needed. Preserve existing regression coverage when converting a test; unrelated legacy tests do not require a bulk rewrite.
- Run the affected suites with `go test` (and `-race` when appropriate). `make test` / `go test ./...` also execute Ginkgo suites; using these commands does not mean writing standard-library test cases.
- Follow [the testing guide](docs/TESTING.md) for examples and verification practices.

## Project Structure
- `cmd/`: Entry points (`main.go`, `agent/`, `fifi-mcp/`, `market-collector/`, `dart-filing-api/`, `dart-filing-worker/`, `dart-filing-worker-cli/`). Existing `market-mcp/` and `dart-filing-mcp/` are compatibility wrappers for the same unified server.
- `internal/`: Core business logic (`domesticstock/`, `domesticfutureoption/`, `companyanalysis/`, `dcf/`, `dartfiling/`, `external/kosis/`, `market/`, `marketreport/`, `fifimcp/`, `db/`).
- `web/`: Vite + React TypeScript SPA frontend.
- `external/`: External reference submodules (`external/open-trading-api`).
- `docs/`: Technical documentation and CLI guides.
- `bin/`: Compiled binaries.

---

# Domain & Implementation Rules

## Market Reports and MCP

- The current report contracts are `docs/PREMARKET_DATA_CONTRACT.md`, `docs/NOW_DATA_CONTRACT.md`, and `docs/SNAPSHOT_DATA_CONTRACT.md`. Initial designs under `docs/archive/` are historical, not implementation instructions.
- One local `fifi` MCP server exposes the existing three market tools and three disclosure tools. `make mcp` / `make mcp-build` are canonical; old market/disclosure MCP commands start the same six-tool server. Keep tool names and request/output contracts stable.
- CLI and market MCP tools share the report service. Market tools save reports/history by default. Disclosure tools read the separately started DART REST API; API unavailability affects only disclosure calls, not initialization, tool listing, or market calls. MCP must not start the API, DART synchronization, or OpenAI analysis automatically.
- Use only actual source prices. Never create synthetic prices, fixed publication cutoffs, or magnitude-based amount conversions in a production report.
- Preserve missing values as null and observed zero as zero. Source observation time is distinct from retrieval time; never manufacture an observation timestamp.
- Facts provenance version 0 is legacy/unverified and excluded from accepted fallback reads. Retain legacy raw evidence; only source-validated observations may receive the current provenance version.
- Logs, including GORM diagnostics, go to stderr. MCP stdout is reserved for protocol messages.
- Worker restarts preserve queues. A new fetch/analyze bootstrap batch requires `DART_WORKER_BOOTSTRAP_FETCH_REPORTS=true`; startup never deletes pending/retry/scheduled/archived tasks.
- See `docs/MARKET_MCP.md` for the single-server local configuration and `docs/MAINTENANCE.md` for local setup, data preservation, and isolated verification.

## KOSPI Master (`kospi_code.mst`)

- Download URL: `https://new.real.download.dws.co.kr/common/master/kospi_code.mst.zip`
- Treat `kospi_code.mst` as a business-date-sensitive master file, not as permanent static metadata.
- The file includes dynamic or date-sensitive fields such as:
  - previous-day volume
  - previous-day market cap
  - base price
  - trading halt / warning flags
  - financial snapshot fields such as net income, ROE, and base year-month
- Do not use `base_date` / `기준년월` as the file freshness date. It is a financial reference period, not the master file generation date.

## Go Implementation Rules

- Relevant code:
  - `internal/domesticstock/market_snapshot.go`
  - `cmd/main.go`
- The Go code must cache the KOSPI master by business date.
- `KOSPI_MASTER_CACHE_FILE` is the base path, not the final dated filename.
- The resolved master cache path format is:
  - `.cache/kospi_code.<YYYYMMDD>.mst`
- The resolved JSON sidecar path format is:
  - `.cache/kospi_code.<YYYYMMDD>.json`
- The resolved actual PBR cache path format is:
  - `.cache/kospi_actual_pbr.<YYYYMMDD>.json`
- The preferred `YYYYMMDD` source is the `market-time` API response when available.
- Do not rely on a simple 24-hour TTL for `kospi_code.mst`.
- If the same business date file already exists, reuse it.
- If the business date changes, download and store a new dated file.

## JSON Sidecar

- When the dated `.mst` cache is loaded in Go, also create a JSON sidecar for human inspection.
- The JSON sidecar currently exports the fields that the Go implementation actually uses:
  - `code`
  - `name`
  - `market_cap`
  - `net_income`
  - `roe`
  - `base_date`
  - `kospi_index_member`
  - `preferred_class`
  - `security_group`
- The JSON also includes:
  - `business_date`
  - `generated_at`
  - `source_path`
  - `record_count`
  - `fields`
  - `schema_version` (currently 2)

## PBR Notes

- KOSPI weighted PBR in Go uses:
  - the dated KOSPI master cache for constituent selection by market cap
  - a separate actual PBR cache file controlled by `KOSPI_ACTUAL_PBR_CACHE_FILE`
- `KOSPI_ACTUAL_PBR_CACHE_FILE` is also a base path, not the final dated filename.
- Preserve the separation between:
  - business-date-scoped KOSPI master cache
  - business-date-scoped actual PBR cache files

## If You Modify This Logic

- Preserve business-date-scoped master cache filenames.
- Preserve JSON sidecar generation.
- Prefer `market-time` derived business dates over plain wall-clock dates when possible.
- Do not collapse the master cache back to a single undated file.

## Index Futures Master (`fo_idx_code_mts.mst`)

- Download URL: `https://new.real.download.dws.co.kr/common/master/fo_idx_code_mts.mst.zip`
- Relevant code:
  - `internal/domesticfutureoption/service.go`
  - `cmd/main.go`
- Treat `fo_idx_code_mts.mst` as a business-date-sensitive master file for domestic index futures/options contract resolution.
- The Go code caches this master by business date.
- `INDEX_FUTURE_MASTER_CACHE_FILE` is the base path, not the final dated filename.
- The resolved master cache path format is:
  - `.cache/fo_idx_code_mts.<YYYYMMDD>.mst`
- The resolved JSON sidecar path format is:
  - `.cache/fo_idx_code_mts.<YYYYMMDD>.json`
- Prefer `market-time` derived business dates when available.
- If the same business date file already exists, reuse it.
- If the business date changes, download and store a new dated file.
- Current near-month KOSPI200 futures resolution in Go prefers:
  - environment override `QUAD_WITCHING_FUTURES_CODE`
  - otherwise futures `info_type == 1` with KOSPI200 underlying `2001` / `201`
  - recent-month classification and short-code prefix `101` are selection preferences, not substitutes for the underlying validation
- Keep the JSON sidecar for human inspection if you change this logic.
- Quadruple witching runnable snapshot now writes a dated JSON export:
  - base path env: `QUAD_WITCHING_SNAPSHOT_JSON_FILE`
  - resolved path format: `.cache/quad_witching_snapshot.<YYYYMMDD>.<futures_code>.json`
- The snapshot JSON should keep:
  - endpoint status (`ok`, `error`, `business_error`, `nil`)
  - `msg_cd`, `msg1`
  - raw response body when available
- `inquire-time-fuopccnl` and `inquire-member` in `internal/domesticfutureoption/service.go` are currently experimental wrappers:
  - they use `FID_INPUT_ISCD`
  - they also send `FID_COND_MRKT_DIV_CODE` when provided
  - keep them non-fatal in `cmd/main.go` until verified against an official sample or production response

## DCF Notes

- Relevant code:
  - `internal/domesticstock/dcf_readiness.go`
  - `internal/domesticstock/dcf_valuation.go`
  - `internal/dcf/engine.go`
  - `docs/DCF_DATA_MAP.md`
- Do not treat all DCF inputs as equally reliable.
- Current implementation tiers are:
  - `exact`: directly from KIS response fields
  - `derived`: computed from KIS response fields
  - `assumed`: env/default assumption layer
- Current practical defaults (explicit option/environment overrides take precedence):
  - `RiskFreeRate`: explicit `DCF_RISK_FREE_RATE`, otherwise `DCF_RISK_FREE_BOND_CODE` + `/uapi/domestic-bond/v1/quotations/inquire-price`, then `comp-interest`
  - `RiskFreeRate` row selection fallback can be narrowed with `DCF_RISK_FREE_NAME_HINT`
  - `Beta`: explicit `DCF_BETA`, otherwise KIS daily-return regression
  - `MarketPremium`: explicit `DCF_MARKET_PREMIUM`, otherwise selected provider (`damodaran` by default, `file` when configured), then assumption fallback
  - `NetDebt`: proxy via `EV - market cap`, with fallback debt-dependence proxy
  - `CostOfDebt`: proxy via non-operating expense and debt proxy, with assumption fallback
- Treat DCF enterprise/equity values as `억원`-scaled financial values unless a different normalization layer is added.
- Final `TargetPrice` must be normalized to `KRW/share` before comparing with `stck_prpr`.
- Advanced engine features now live in `internal/dcf/advanced.go`:
  - `ReverseDCF`
  - `MonteCarlo`
- `MonteCarlo` JSON export is written from `cmd/main.go` via `internal/dcf/export.go`
- `DCF_MONTE_CARLO_JSON_FILE` is a base path; the runnable example resolves it to a business-date and symbol scoped JSON filename.
- If you change DCF logic, keep the distinction between `exact`, `derived`, and `assumed` visible in code and output.

## Git Commit Rule
- **Do not create git commits without explicit user request.**
- Only modify or edit files in the workspace. Let the user review and create commits manually, unless the user explicitly requests a git commit.
