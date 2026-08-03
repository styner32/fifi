package companyanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fifi/internal/testhelpers"
)

func TestAnalyzeBuildsUSDValuationFromExternalInputs(t *testing.T) {
	transport := testhelpers.NewMockTransport()
	t.Cleanup(func() {
		if err := transport.Verify(); err != nil {
			t.Fatal(err)
		}
	})

	transport.New("https://sec.test").
		Get("/company_tickers.json").
		Reply(http.StatusOK).
		JSON(map[string]any{
			"0": map[string]any{
				"cik_str": 1045810,
				"ticker":  "NVDA",
				"title":   "NVIDIA CORP",
			},
		})

	transport.New("https://sec.test").
		Get("/companyfacts/CIK0001045810.json").
		Reply(http.StatusOK).
		Body(buildCompanyFactsPayload(t))

	transport.New("https://fred.test").
		Get("/dgs10.csv").
		Reply(http.StatusOK).
		BodyString("observation_date,DGS10\n2026-03-16,4.10\n2026-03-17,4.15\n")

	transport.New("https://stooq.test").
		Get("/?i=d&s=nvda.us").
		Reply(http.StatusOK).
		BodyString(buildStooqCSV("2026-01-01", 100, 40, 0.0016))

	transport.New("https://stooq.test").
		Get("/?i=d&s=spy.us").
		Reply(http.StatusOK).
		BodyString(buildStooqCSV("2026-01-01", 200, 40, 0.0011))

	svc := NewService(&http.Client{Transport: transport}, Config{
		SECTickersURL:       "https://sec.test/company_tickers.json",
		SECCompanyFactsBase: "https://sec.test/companyfacts",
		FRED10YearURL:       "https://fred.test/dgs10.csv",
		StooqBaseURL:        "https://stooq.test",
		SECUserAgent:        "test-agent",
	})

	marketPremium := 0.055
	result, err := svc.Analyze(context.Background(), "NVDA", AnalysisOptions{
		BenchmarkSymbol:  "SPY",
		MarketPremium:    &marketPremium,
		BetaLookbackDays: 30,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if result.Symbol != "NVDA" {
		t.Fatalf("unexpected symbol %q", result.Symbol)
	}
	if result.CompanyName != "NVIDIA CORP" {
		t.Fatalf("unexpected company name %q", result.CompanyName)
	}
	if result.Quote.Currency != "USD" {
		t.Fatalf("unexpected quote currency %q", result.Quote.Currency)
	}
	if result.Quote.Price <= 0 {
		t.Fatalf("expected positive quote price, got %.4f", result.Quote.Price)
	}
	if result.Market.RiskFreeRate <= 0.041 || result.Market.RiskFreeRate >= 0.042 {
		t.Fatalf("unexpected risk free rate %.6f", result.Market.RiskFreeRate)
	}
	if result.Market.Beta <= 0 {
		t.Fatalf("expected positive beta, got %.6f", result.Market.Beta)
	}
	if result.Valuation == nil {
		t.Fatal("valuation is nil")
	}
	if result.Valuation.TargetPriceScale != 1 {
		t.Fatalf("unexpected target price scale %.2f", result.Valuation.TargetPriceScale)
	}
	if result.Valuation.TargetPriceUnit != "USD/share" {
		t.Fatalf("unexpected target price unit %q", result.Valuation.TargetPriceUnit)
	}
	if result.KeyMetrics.MarketCap <= 0 {
		t.Fatalf("expected positive market cap, got %.2f", result.KeyMetrics.MarketCap)
	}
}

func TestAnalyzeFallsBackToCachedSECDataWhenFairAccessPageIsReturned(t *testing.T) {
	cacheDir := t.TempDir()
	tickersCachePath := filepath.Join(cacheDir, "sec_company_tickers.json")
	factsCachePath := filepath.Join(cacheDir, "sec_companyfacts.json")
	if err := os.WriteFile(tickersCachePath, []byte(`{"0":{"cik_str":1045810,"ticker":"NVDA","title":"NVIDIA CORP"}}`), 0o600); err != nil {
		t.Fatalf("failed to seed tickers cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "sec_companyfacts.0001045810.json"), buildCompanyFactsPayload(t), 0o600); err != nil {
		t.Fatalf("failed to seed company facts cache: %v", err)
	}

	transport := testhelpers.NewMockTransport()
	t.Cleanup(func() {
		if err := transport.Verify(); err != nil {
			t.Fatal(err)
		}
	})

	blockedBody := `<body><h1>Automated access to our sites must comply with SEC.gov's Privacy and Security Policy.</h1></body>`
	transport.New("https://sec.test").
		Get("/company_tickers.json").
		Reply(http.StatusOK).
		BodyString(blockedBody)
	transport.New("https://sec.test").
		Get("/companyfacts/CIK0001045810.json").
		Reply(http.StatusOK).
		BodyString(blockedBody)
	transport.New("https://fred.test").
		Get("/dgs10.csv").
		Reply(http.StatusOK).
		BodyString("observation_date,DGS10\n2026-03-17,4.15\n")
	transport.New("https://stooq.test").
		Get("/?i=d&s=nvda.us").
		Reply(http.StatusOK).
		BodyString(buildStooqCSV("2026-01-01", 100, 40, 0.0016))
	transport.New("https://stooq.test").
		Get("/?i=d&s=spy.us").
		Reply(http.StatusOK).
		BodyString(buildStooqCSV("2026-01-01", 200, 40, 0.0011))

	svc := NewService(&http.Client{Transport: transport}, Config{
		SECTickersURL:            "https://sec.test/company_tickers.json",
		SECCompanyFactsBase:      "https://sec.test/companyfacts",
		FRED10YearURL:            "https://fred.test/dgs10.csv",
		StooqBaseURL:             "https://stooq.test",
		SECUserAgent:             "test-agent contact@example.com",
		SECTickersCachePath:      tickersCachePath,
		SECCompanyFactsCachePath: factsCachePath,
	})

	marketPremium := 0.055
	result, err := svc.Analyze(context.Background(), "NVDA", AnalysisOptions{
		BenchmarkSymbol:  "SPY",
		MarketPremium:    &marketPremium,
		BetaLookbackDays: 30,
	})
	if err != nil {
		t.Fatalf("Analyze returned error with cached SEC data: %v", err)
	}
	if result.CompanyName != "NVIDIA CORP" {
		t.Fatalf("unexpected company name %q", result.CompanyName)
	}
}

func buildCompanyFactsPayload(t *testing.T) []byte {
	t.Helper()

	payload := map[string]any{
		"cik":        1045810,
		"entityName": "NVIDIA CORP",
		"facts": map[string]any{
			"dei": map[string]any{
				"EntityCommonStockSharesOutstanding": map[string]any{
					"units": map[string]any{
						"shares": []map[string]any{
							annualObservation("2024-01-01", "2024-12-31", 100, 2024, "2025-02-01", "10-K"),
							annualObservation("2025-01-01", "2025-12-31", 100, 2025, "2026-02-01", "10-K"),
							instantObservation("2026-02-15", 102, 2026, "2026-02-15", "10-Q"),
						},
					},
				},
			},
			"us-gaap": map[string]any{
				"RevenueFromContractWithCustomerExcludingAssessedTax": conceptUSD(
					annualObservation("2023-01-01", "2023-12-31", 800, 2023, "2024-02-01", "10-K"),
					annualObservation("2024-01-01", "2024-12-31", 1000, 2024, "2025-02-01", "10-K"),
					annualObservation("2025-01-01", "2025-12-31", 1200, 2025, "2026-02-01", "10-K"),
				),
				"OperatingIncomeLoss": conceptUSD(
					annualObservation("2023-01-01", "2023-12-31", 160, 2023, "2024-02-01", "10-K"),
					annualObservation("2024-01-01", "2024-12-31", 240, 2024, "2025-02-01", "10-K"),
					annualObservation("2025-01-01", "2025-12-31", 300, 2025, "2026-02-01", "10-K"),
				),
				"NetIncomeLoss": conceptUSD(
					annualObservation("2023-01-01", "2023-12-31", 120, 2023, "2024-02-01", "10-K"),
					annualObservation("2024-01-01", "2024-12-31", 200, 2024, "2025-02-01", "10-K"),
					annualObservation("2025-01-01", "2025-12-31", 250, 2025, "2026-02-01", "10-K"),
				),
				"IncomeTaxExpenseBenefit": conceptUSD(
					annualObservation("2023-01-01", "2023-12-31", 30, 2023, "2024-02-01", "10-K"),
					annualObservation("2024-01-01", "2024-12-31", 40, 2024, "2025-02-01", "10-K"),
					annualObservation("2025-01-01", "2025-12-31", 50, 2025, "2026-02-01", "10-K"),
				),
				"EffectiveIncomeTaxRateContinuingOperations": conceptPure(
					annualObservation("2023-01-01", "2023-12-31", 0.20, 2023, "2024-02-01", "10-K"),
					annualObservation("2024-01-01", "2024-12-31", 0.167, 2024, "2025-02-01", "10-K"),
					annualObservation("2025-01-01", "2025-12-31", 0.167, 2025, "2026-02-01", "10-K"),
				),
				"DepreciationDepletionAndAmortization": conceptUSD(
					annualObservation("2023-01-01", "2023-12-31", 40, 2023, "2024-02-01", "10-K"),
					annualObservation("2024-01-01", "2024-12-31", 50, 2024, "2025-02-01", "10-K"),
					annualObservation("2025-01-01", "2025-12-31", 60, 2025, "2026-02-01", "10-K"),
				),
				"AssetsCurrent": conceptUSD(
					instantObservation("2023-12-31", 400, 2023, "2024-02-01", "10-K"),
					instantObservation("2024-12-31", 500, 2024, "2025-02-01", "10-K"),
					instantObservation("2025-12-31", 600, 2025, "2026-02-01", "10-K"),
				),
				"LiabilitiesCurrent": conceptUSD(
					instantObservation("2023-12-31", 180, 2023, "2024-02-01", "10-K"),
					instantObservation("2024-12-31", 200, 2024, "2025-02-01", "10-K"),
					instantObservation("2025-12-31", 220, 2025, "2026-02-01", "10-K"),
				),
				"Assets": conceptUSD(
					instantObservation("2023-12-31", 1200, 2023, "2024-02-01", "10-K"),
					instantObservation("2024-12-31", 1400, 2024, "2025-02-01", "10-K"),
					instantObservation("2025-12-31", 1600, 2025, "2026-02-01", "10-K"),
				),
				"CashAndCashEquivalentsAtCarryingValue": conceptUSD(
					instantObservation("2023-12-31", 90, 2023, "2024-02-01", "10-K"),
					instantObservation("2024-12-31", 120, 2024, "2025-02-01", "10-K"),
					instantObservation("2025-12-31", 150, 2025, "2026-02-01", "10-K"),
				),
				"LongTermDebt": conceptUSD(
					instantObservation("2023-12-31", 240, 2023, "2024-02-01", "10-K"),
					instantObservation("2024-12-31", 220, 2024, "2025-02-01", "10-K"),
					instantObservation("2025-12-31", 200, 2025, "2026-02-01", "10-K"),
				),
				"LongTermDebtCurrent": conceptUSD(
					instantObservation("2023-12-31", 20, 2023, "2024-02-01", "10-K"),
					instantObservation("2024-12-31", 20, 2024, "2025-02-01", "10-K"),
					instantObservation("2025-12-31", 20, 2025, "2026-02-01", "10-K"),
				),
				"StockholdersEquity": conceptUSD(
					instantObservation("2023-12-31", 650, 2023, "2024-02-01", "10-K"),
					instantObservation("2024-12-31", 780, 2024, "2025-02-01", "10-K"),
					instantObservation("2025-12-31", 900, 2025, "2026-02-01", "10-K"),
				),
				"InterestExpense": conceptUSD(
					annualObservation("2023-01-01", "2023-12-31", 18, 2023, "2024-02-01", "10-K"),
					annualObservation("2024-01-01", "2024-12-31", 16, 2024, "2025-02-01", "10-K"),
					annualObservation("2025-01-01", "2025-12-31", 14, 2025, "2026-02-01", "10-K"),
				),
				"PropertyPlantAndEquipmentNet": conceptUSD(
					instantObservation("2023-12-31", 430, 2023, "2024-02-01", "10-K"),
					instantObservation("2024-12-31", 460, 2024, "2025-02-01", "10-K"),
					instantObservation("2025-12-31", 500, 2025, "2026-02-01", "10-K"),
				),
			},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return raw
}

func conceptUSD(observations ...map[string]any) map[string]any {
	return map[string]any{
		"units": map[string]any{
			"USD": observations,
		},
	}
}

func conceptPure(observations ...map[string]any) map[string]any {
	return map[string]any{
		"units": map[string]any{
			"pure": observations,
		},
	}
}

func annualObservation(start string, end string, value float64, fiscalYear int, filed string, form string) map[string]any {
	return map[string]any{
		"start": start,
		"end":   end,
		"val":   value,
		"fy":    fiscalYear,
		"fp":    "FY",
		"form":  form,
		"filed": filed,
		"accn":  fmt.Sprintf("%d-annual", fiscalYear),
		"frame": fmt.Sprintf("CY%d", fiscalYear),
	}
}

func instantObservation(end string, value float64, fiscalYear int, filed string, form string) map[string]any {
	return map[string]any{
		"end":   end,
		"val":   value,
		"fy":    fiscalYear,
		"fp":    "FY",
		"form":  form,
		"filed": filed,
		"accn":  fmt.Sprintf("%d-instant", fiscalYear),
	}
}

func buildStooqCSV(start string, firstClose float64, count int, drift float64) string {
	startDate, _ := time.Parse("2006-01-02", start)
	var builder strings.Builder
	builder.WriteString("Date,Open,High,Low,Close,Volume\n")
	closeValue := firstClose
	for i := 0; i < count; i++ {
		returnStep := drift + (float64(i%5) * 0.0003)
		closeValue = closeValue * (1 + returnStep)
		currentDate := startDate.AddDate(0, 0, i)
		builder.WriteString(fmt.Sprintf(
			"%s,%.4f,%.4f,%.4f,%.4f,%d\n",
			currentDate.Format("2006-01-02"),
			closeValue*0.99,
			closeValue*1.01,
			closeValue*0.98,
			closeValue,
			1000000+i,
		))
	}
	return builder.String()
}
