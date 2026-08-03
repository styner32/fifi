# DCF Data Map

## Current Go Coverage

The current Go implementation can now build a practical end-to-end DCF valuation from KIS domestic stock APIs plus a small assumption layer.

Implemented code:
- `go/internal/domesticstock/dcf_readiness.go`
- `go/internal/domesticstock/dcf_valuation.go`
- `go/internal/dcf/engine.go`
- `go/internal/dcf/advanced.go`
- `go/cmd/main.go`

## Input Mapping

| DCF field | Current status | Source | Notes |
| --- | --- | --- | --- |
| `Revenue` | exact | `income-statement.output.sale_account` | Latest annual revenue |
| `EBIT` | exact | `income-statement.output.bsop_prti` | Uses operating profit as EBIT proxy |
| `EffectiveTax` | derived | `income-statement.output.(op_prfi, thtr_ntin)` | `1 - thtr_ntin / op_prfi` |
| `DnA` | exact | `income-statement.output.depr_cost` | Depreciation field confirmed |
| `CapEx` | derived | `balance-sheet.output.fxas` + `income-statement.output.depr_cost` | Proxy: `delta fixed assets + depreciation` |
| `ChangeInNWC` | derived | `balance-sheet.output.(cras, flow_lblt)` | Proxy: delta of `(current assets - current liabilities)` |
| `SharesOut` | exact | `inquire-price.output.lstn_stcn` | Listed shares |
| `NetDebt` | derived | `other-major-ratios.output.(ebitda, ev_ebitda)` + `inquire-price.output.hts_avls` | Proxy: `EV - market cap`; fallback uses debt dependence |
| `RiskFreeRate` | exact / assumed | `domestic-bond.inquire-price.output.(stnd_iscd, ernn_rate)` or `comp-interest.output.(hts_kor_isnm, bond_mnrt_prpr)` or `DCF_RISK_FREE_RATE` | Prefers code-based domestic bond quote via `DCF_RISK_FREE_BOND_CODE`; falls back to `comp-interest` name matching |
| `Beta` | derived / assumed | `inquire-daily-itemchartprice` + `inquire-index-daily-price` or `DCF_BETA` | Rolling beta from matched daily returns |
| `MarketPremium` | exact / assumed | Damodaran implied ERP provider, `DCF_MARKET_PREMIUM`, or `DCF_MARKET_PREMIUM_FILE` | Prefers external provider; falls back to `5.5%` assumption only when provider is unavailable |
| `CostOfDebt` | derived / assumed | `income-statement.output.bsop_non_expn` + `stability-ratio.output.bram_depn` or `DCF_COST_OF_DEBT` | Proxy from non-operating expense over debt proxy; fallback uses risk-free + spread |
| `EquityWeight` | derived | market cap + net debt proxy | Prefers market-value capital structure |
| `DebtWeight` | derived | market cap + net debt proxy | Falls back to book-capital proxy |
| `TargetPrice` | derived | `equity_value / shares_out * 100,000,000` | Internal enterprise/equity values are treated as `억원`; final target price is normalized to `KRW/share` |

## What This Means

- `FCF projection`: possible
- `WACC`: possible
- `Enterprise Value`: possible
- `Target Price`: possible
- `Reverse DCF`: possible
- `Monte Carlo`: possible

Important caveat:
- `NetDebt` and `CostOfDebt` remain proxy-style inputs.
- `MarketPremium` is now provider-backed by default, but it is still an external market assumption input rather than a KIS-native company fact.

## API Calls Used By DCF Readiness / Valuation

Required company calls:
1. `/uapi/domestic-stock/v1/quotations/inquire-price`
2. `/uapi/domestic-stock/v1/finance/balance-sheet`
3. `/uapi/domestic-stock/v1/finance/income-statement`

Additional calls for market / proxy inputs:
4. `/uapi/domestic-stock/v1/finance/other-major-ratios`
5. `/uapi/domestic-stock/v1/finance/stability-ratio`
6. `/uapi/domestic-bond/v1/quotations/inquire-price`
7. `/uapi/domestic-stock/v1/quotations/comp-interest`
8. `/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice`
9. `/uapi/domestic-stock/v1/quotations/inquire-index-daily-price`
10. Damodaran home page (`https://pages.stern.nyu.edu/adamodar/New_Home_Page/home.htm`) for implied ERP

## Assumption / Override Environment Variables

- `DCF_RISK_FREE_RATE`
- `DCF_RISK_FREE_BOND_CODE`
- `DCF_RISK_FREE_BOND_MARKET_DIV_CODE`
- `DCF_RISK_FREE_NAME_HINT`
- `DCF_BETA`
- `DCF_MARKET_PREMIUM`
- `DCF_MARKET_PREMIUM_PROVIDER`
- `DCF_MARKET_PREMIUM_FILE`
- `DCF_MARKET_PREMIUM_URL`
- `DCF_COST_OF_DEBT`
- `DCF_NET_DEBT`
- `DCF_FORECAST_YEARS`
- `DCF_TERMINAL_GROWTH`
- `DCF_INDEX_CODE`
- `DCF_BETA_LOOKBACK_DAYS`
- `DCF_CREDIT_SPREAD`
- `DCF_MONTE_CARLO_ITERATIONS`
- `DCF_MONTE_CARLO_WORKERS`
- `DCF_MONTE_CARLO_GROWTH_STDDEV`
- `DCF_MONTE_CARLO_WACC_STDDEV`
- `DCF_MONTE_CARLO_TERMINAL_STDDEV`
- `DCF_MONTE_CARLO_JSON_FILE`
