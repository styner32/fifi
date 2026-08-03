package main

import (
	"os"
	"strings"
	"time"

	"github.com/fifi/internal/commodityfuture"
	"github.com/fifi/internal/envcfg"
	"github.com/fifi/internal/shippingindex"
)

type appConfig struct {
	appKey                                  string
	secretKey                               string
	userAgent                               string
	baseURL                                 string
	tokenCachePath                          string
	targetSymbol                            string
	kospiProxyTargetCoverage                float64
	ewySymbol                               string
	exchangeRateMarketDivCode               string
	exchangeRateSymbol                      string
	copperFutureTicker                      string
	copperFutureProductCode                 string
	copperFutureCMEProductID                string
	goldFutureTicker                        string
	goldFutureProductCode                   string
	goldFutureCMEProductID                  string
	silverFutureTicker                      string
	silverFutureProductCode                 string
	silverFutureCMEProductID                string
	commodityFutureProviderOrder            []string
	commodityFutureUserAgent                string
	shippingIndexSymbols                    []string
	companyAnalysisSymbol                   string
	companyAnalysisBenchmarkSymbol          string
	companyAnalysisForecastYears            int
	companyAnalysisTerminalGrowth           float64
	companyAnalysisBetaLookbackDays         int
	companyAnalysisJSONFile                 string
	companyAnalysisSECTickersCacheFile      string
	companyAnalysisSECCompanyFactsCacheFile string
	companyAnalysisSECUserAgent             string
	quadWitchingLookaheadDays               int
	quadWitchingGraceDays                   int
	quadWitchingForce                       bool
	quadWitchingFuturesMarketDivCode        string
	quadWitchingFuturesHourClsCode          string
	quadWitchingFuturesIncludePastData      string
	quadWitchingFuturesIncludeFakeTick      string
	quadWitchingFuturesInputHour            string
	quadWitchingInvestorSymbol              string
	quadWitchingKOSPI200InvestorSymbol      string
	quadWitchingForeignMarketDivCode        string
	quadWitchingForeignScreenDivCode        string
	quadWitchingForeignInputISCD            string
	quadWitchingForeignDivClsCode           string
	quadWitchingForeignRankSortCode         string
	quadWitchingForeignEtcClsCode           string
	quadWitchingAuctionSymbol               string
	quadWitchingSnapshotJSONFile            string
	dcfMonteCarloJSONFile                   string
	dcfMonteCarloIterations                 int
	dcfMonteCarloWorkers                    int
	dcfMonteCarloGrowthStdDev               float64
	dcfMonteCarloWACCStdDev                 float64
	dcfMonteCarloTerminalStdDev             float64
}

func loadAppConfig(now time.Time) appConfig {
	targetSymbol := getOrDefault("TARGET_SYMBOL", "005930")
	companyAnalysisSECUserAgent := strings.TrimSpace(os.Getenv("COMPANY_ANALYSIS_SEC_USER_AGENT"))
	if companyAnalysisSECUserAgent == "" {
		companyAnalysisSECContactEmail := strings.TrimSpace(os.Getenv("COMPANY_ANALYSIS_SEC_CONTACT_EMAIL"))
		if companyAnalysisSECContactEmail != "" {
			companyAnalysisSECUserAgent = "open-trading-api/1.0 " + companyAnalysisSECContactEmail
		}
	}
	if companyAnalysisSECUserAgent == "" {
		companyAnalysisSECUserAgent = "open-trading-api/1.0 contact@example.com"
	}

	return appConfig{
		appKey:                                  os.Getenv("APP_KEY"),
		secretKey:                               os.Getenv("APP_SECRET"),
		userAgent:                               os.Getenv("USER_AGENT"),
		baseURL:                                 "https://openapi.koreainvestment.com:9443",
		tokenCachePath:                          getOrDefault("AUTH_TOKEN_FILE", ".auth_token.json"),
		targetSymbol:                            targetSymbol,
		kospiProxyTargetCoverage:                getFloatOrDefault("KOSPI_PROXY_PBR_TARGET_COVERAGE", 0.80),
		ewySymbol:                               getOrDefault("EWY_SYMBOL", "EWY"),
		exchangeRateMarketDivCode:               getOrDefault("EXCHANGE_RATE_MARKET_DIV_CODE", "X"),
		exchangeRateSymbol:                      getOrDefault("EXCHANGE_RATE_SYMBOL", "USDKRW"),
		copperFutureTicker:                      getOrDefault("COPPER_FUTURE_TICKER", "HG=F"),
		copperFutureProductCode:                 commodityfuture.NormalizeProductCode(getOrDefault("COPPER_FUTURE_PRODUCT_CODE", getOrDefault("COPPER_FUTURE_TICKER", "HG=F"))),
		copperFutureCMEProductID:                strings.TrimSpace(os.Getenv("COPPER_FUTURE_CME_PRODUCT_ID")),
		goldFutureTicker:                        getOrDefault("GOLD_FUTURE_TICKER", "GC=F"),
		goldFutureProductCode:                   commodityfuture.NormalizeProductCode(getOrDefault("GOLD_FUTURE_PRODUCT_CODE", getOrDefault("GOLD_FUTURE_TICKER", "GC=F"))),
		goldFutureCMEProductID:                  strings.TrimSpace(os.Getenv("GOLD_FUTURE_CME_PRODUCT_ID")),
		silverFutureTicker:                      getOrDefault("SILVER_FUTURE_TICKER", "SI=F"),
		silverFutureProductCode:                 commodityfuture.NormalizeProductCode(getOrDefault("SILVER_FUTURE_PRODUCT_CODE", getOrDefault("SILVER_FUTURE_TICKER", "SI=F"))),
		silverFutureCMEProductID:                strings.TrimSpace(os.Getenv("SILVER_FUTURE_CME_PRODUCT_ID")),
		commodityFutureProviderOrder:            splitCSV(getOrDefault("COMMODITY_FUTURE_PROVIDER_ORDER", commodityfuture.DefaultProviderOrderCSV)),
		commodityFutureUserAgent:                strings.TrimSpace(getOrDefault("COMMODITY_FUTURE_USER_AGENT", commodityfuture.DefaultUserAgent)),
		shippingIndexSymbols:                    splitCSV(getOrDefault("SHIPPING_INDEX_SYMBOLS", strings.Join(shippingindex.DefaultSymbols(), ","))),
		companyAnalysisSymbol:                   strings.ToUpper(strings.TrimSpace(getOrDefault("COMPANY_ANALYSIS_SYMBOL", "NVDA"))),
		companyAnalysisBenchmarkSymbol:          strings.ToUpper(strings.TrimSpace(getOrDefault("COMPANY_ANALYSIS_BENCHMARK_SYMBOL", "SPY"))),
		companyAnalysisForecastYears:            getIntOrDefault("COMPANY_ANALYSIS_FORECAST_YEARS", 5),
		companyAnalysisTerminalGrowth:           getFloatOrDefault("COMPANY_ANALYSIS_TERMINAL_GROWTH", 0.025),
		companyAnalysisBetaLookbackDays:         getIntOrDefault("COMPANY_ANALYSIS_BETA_LOOKBACK_DAYS", 252),
		companyAnalysisJSONFile:                 getOrDefault("COMPANY_ANALYSIS_JSON_FILE", ".cache/company_analysis.json"),
		companyAnalysisSECTickersCacheFile:      getOrDefault("COMPANY_ANALYSIS_SEC_TICKERS_CACHE_FILE", ".cache/sec_company_tickers.json"),
		companyAnalysisSECCompanyFactsCacheFile: getOrDefault("COMPANY_ANALYSIS_SEC_COMPANYFACTS_CACHE_FILE", ".cache/sec_companyfacts.json"),
		companyAnalysisSECUserAgent:             companyAnalysisSECUserAgent,
		quadWitchingLookaheadDays:               getIntOrDefault("QUAD_WITCHING_LOOKAHEAD_DAYS", 7),
		quadWitchingGraceDays:                   getIntOrDefault("QUAD_WITCHING_GRACE_DAYS", 0),
		quadWitchingForce:                       getBoolOrDefault("QUAD_WITCHING_FORCE", false),
		quadWitchingFuturesMarketDivCode:        getOrDefault("QUAD_WITCHING_FUTURES_MARKET_DIV_CODE", "F"),
		quadWitchingFuturesHourClsCode:          getOrDefault("QUAD_WITCHING_FUTURES_HOUR_CLS_CODE", "60"),
		quadWitchingFuturesIncludePastData:      getOrDefault("QUAD_WITCHING_FUTURES_INCLUDE_PAST_DATA", "Y"),
		quadWitchingFuturesIncludeFakeTick:      getOrDefault("QUAD_WITCHING_FUTURES_INCLUDE_FAKE_TICK", "N"),
		quadWitchingFuturesInputHour:            getOrDefault("QUAD_WITCHING_FUTURES_INPUT_HOUR", now.Format("150405")),
		quadWitchingInvestorSymbol:              getOrDefault("QUAD_WITCHING_INVESTOR_SYMBOL", "0001"),
		quadWitchingKOSPI200InvestorSymbol:      getOrDefault("QUAD_WITCHING_KOSPI200_INVESTOR_SYMBOL", "2001"),
		quadWitchingForeignMarketDivCode:        getOrDefault("QUAD_WITCHING_FOREIGN_MARKET_DIV_CODE", "V"),
		quadWitchingForeignScreenDivCode:        getOrDefault("QUAD_WITCHING_FOREIGN_SCREEN_DIV_CODE", "16449"),
		quadWitchingForeignInputISCD:            getOrDefault("QUAD_WITCHING_FOREIGN_INPUT_ISCD", "0000"),
		quadWitchingForeignDivClsCode:           getOrDefault("QUAD_WITCHING_FOREIGN_DIV_CLS_CODE", "1"),
		quadWitchingForeignRankSortCode:         getOrDefault("QUAD_WITCHING_FOREIGN_RANK_SORT_CODE", "0"),
		quadWitchingForeignEtcClsCode:           getOrDefault("QUAD_WITCHING_FOREIGN_ETC_CLS_CODE", "1"),
		quadWitchingAuctionSymbol:               getOrDefault("QUAD_WITCHING_AUCTION_SYMBOL", targetSymbol),
		quadWitchingSnapshotJSONFile:            getOrDefault("QUAD_WITCHING_SNAPSHOT_JSON_FILE", ".cache/quad_witching_snapshot.json"),
		dcfMonteCarloJSONFile:                   getOrDefault("DCF_MONTE_CARLO_JSON_FILE", ".cache/dcf_monte_carlo.json"),
		dcfMonteCarloIterations:                 getIntOrDefault("DCF_MONTE_CARLO_ITERATIONS", 2000),
		dcfMonteCarloWorkers:                    getIntOrDefault("DCF_MONTE_CARLO_WORKERS", 0),
		dcfMonteCarloGrowthStdDev:               getFloatOrDefault("DCF_MONTE_CARLO_GROWTH_STDDEV", 0.02),
		dcfMonteCarloWACCStdDev:                 getFloatOrDefault("DCF_MONTE_CARLO_WACC_STDDEV", 0.01),
		dcfMonteCarloTerminalStdDev:             getFloatOrDefault("DCF_MONTE_CARLO_TERMINAL_STDDEV", 0.005),
	}
}

func newAppDates(now time.Time) appDates {
	return appDates{
		now:                  now,
		today:                now.Format("20060102"),
		fromDate:             now.AddDate(0, -6, 0).Format("20060102"),
		exchangeRateFromDate: now.AddDate(0, 0, -14).Format("20060102"),
	}
}

func getOrDefault(key string, defaultValue string) string {
	return envcfg.Get(key, defaultValue)
}

func getFloatOrDefault(key string, defaultValue float64) float64 {
	return envcfg.Float(key, defaultValue)
}

func getIntOrDefault(key string, defaultValue int) int {
	return envcfg.Int(key, defaultValue)
}

func getBoolOrDefault(key string, defaultValue bool) bool {
	return envcfg.Bool(key, defaultValue)
}

func getOptionalFloat(key string) *float64 {
	return envcfg.OptionalFloat(key)
}
