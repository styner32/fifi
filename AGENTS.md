# Project Notes

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
  - `go/internal/domesticstock/market_status.go`
  - `go/cmd/main.go`
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
- The JSON also includes:
  - `business_date`
  - `generated_at`
  - `source_path`
  - `record_count`
  - `fields`

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
  - `go/internal/domesticfutureoption/service.go`
  - `go/cmd/main.go`
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
  - otherwise `info_type == 1` and short code prefix `101`
  - then `month_class_code == 1` (recent month)
- Keep the JSON sidecar for human inspection if you change this logic.
- Quadruple witching runnable snapshot now writes a dated JSON export:
  - base path env: `QUAD_WITCHING_SNAPSHOT_JSON_FILE`
  - resolved path format: `.cache/quad_witching_snapshot.<YYYYMMDD>.<futures_code>.json`
- The snapshot JSON should keep:
  - endpoint status (`ok`, `error`, `business_error`, `nil`)
  - `msg_cd`, `msg1`
  - raw response body when available
- `inquire-time-fuopccnl` and `inquire-member` in `go/internal/domesticfutureoption/service.go` are currently experimental wrappers:
  - they use `FID_INPUT_ISCD`
  - they also send `FID_COND_MRKT_DIV_CODE` when provided
  - keep them non-fatal in `go/cmd/main.go` until verified against an official sample or production response

## DCF Notes

- Relevant code:
  - `go/internal/domesticstock/dcf_readiness.go`
  - `go/internal/domesticstock/dcf_valuation.go`
  - `go/internal/dcf/engine.go`
  - `go/DCF_DATA_MAP.md`
- Do not treat all DCF inputs as equally reliable.
- Current implementation tiers are:
  - `exact`: directly from KIS response fields
  - `derived`: computed from KIS response fields
  - `assumed`: env/default assumption layer
- Current practical defaults:
  - `RiskFreeRate`: prefer `DCF_RISK_FREE_BOND_CODE` + `/uapi/domestic-bond/v1/quotations/inquire-price`, then `comp-interest`, then env override
  - `RiskFreeRate` row selection fallback can be narrowed with `DCF_RISK_FREE_NAME_HINT`
  - `Beta`: prefer KIS daily-return regression, then env override
  - `MarketPremium`: prefer external provider, then env/file override, then assumption fallback
  - `NetDebt`: proxy via `EV - market cap`, with fallback debt-dependence proxy
  - `CostOfDebt`: proxy via non-operating expense and debt proxy, with assumption fallback
- Treat DCF enterprise/equity values as `억원`-scaled financial values unless a different normalization layer is added.
- Final `TargetPrice` must be normalized to `KRW/share` before comparing with `stck_prpr`.
- Advanced engine features now live in `go/internal/dcf/advanced.go`:
  - `ReverseDCF`
  - `MonteCarlo`
- `MonteCarlo` JSON export is written from `go/cmd/main.go` via `go/internal/dcf/export.go`
- `DCF_MONTE_CARLO_JSON_FILE` is a base path; the runnable example resolves it to a business-date and symbol scoped JSON filename.
- If you change DCF logic, keep the distinction between `exact`, `derived`, and `assumed` visible in code and output.
