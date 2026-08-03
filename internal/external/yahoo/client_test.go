package yahoo

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/fifi/internal/testhelpers"
)

func TestClientGetQuotes(t *testing.T) {
	transport := testhelpers.NewMockTransport()
	
	// Mock ^N225
	transport.New("https://example.test").
		Get("/v8/finance/chart/%5EN225?interval=1d&range=1d").
		MatchHeader("User-Agent", "unit-test").
		Reply(http.StatusOK).
		BodyString(`{
  "chart": {
    "result": [
      {
        "meta": {
          "symbol": "^N225",
          "shortName": "Nikkei 225",
          "regularMarketPrice": 38500.25,
          "chartPreviousClose": 38600.25
        }
      }
    ],
    "error": null
  }
}`)

	// Mock NQ=F
	transport.New("https://example.test").
		Get("/v8/finance/chart/NQ%3DF?interval=1d&range=1d").
		MatchHeader("User-Agent", "unit-test").
		Reply(http.StatusOK).
		BodyString(`{
  "chart": {
    "result": [
      {
        "meta": {
          "symbol": "NQ=F",
          "shortName": "Nasdaq 100 Futures",
          "regularMarketPrice": 18200.0,
          "chartPreviousClose": 18100.0
        }
      }
    ],
    "error": null
  }
}`)

	client := NewClient(&http.Client{Transport: transport}, Config{
		BaseURL:   "https://example.test",
		UserAgent: "unit-test",
	})
	quotes, err := client.GetQuotes(context.Background(), []string{"^N225", "NQ=F"})
	if err != nil {
		t.Fatalf("GetQuotes() error = %v", err)
	}
	if quotes["^N225"].Price != 38500.25 {
		t.Fatalf("N225 price = %v", quotes["^N225"].Price)
	}
	
	// Change percent for NQ=F: (18200 - 18100) / 18100 * 100 = 0.552486...
	change := quotes["NQ=F"].ChangePercent
	if change < 0.55 || change > 0.56 {
		t.Fatalf("NQ=F change = %v", quotes["NQ=F"].ChangePercent)
	}
	if err := transport.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestClientGetQuotesReturnsPartialMissingSymbolError(t *testing.T) {
	transport := testhelpers.NewMockTransport()
	transport.New("https://example.test").
		Get("/v8/finance/chart/KRW%3DX?interval=1d&range=1d").
		Reply(http.StatusOK).
		BodyString(`{
  "chart": {
    "result": [
      {
        "meta": {
          "symbol": "KRW=X",
          "regularMarketPrice": 1494.2,
          "chartPreviousClose": 1490.0
        }
      }
    ],
    "error": null
  }
}`)

	// ^TNX will fail with 404
	transport.New("https://example.test").
		Get("/v8/finance/chart/%5ETNX?interval=1d&range=1d").
		Reply(http.StatusNotFound).
		BodyString(`Not Found`)

	client := NewClient(&http.Client{Transport: transport}, Config{BaseURL: "https://example.test"})
	quotes, err := client.GetQuotes(context.Background(), []string{"KRW=X", "^TNX"})
	if err == nil || !strings.Contains(err.Error(), "^TNX") {
		t.Fatalf("GetQuotes() error = %v, want missing ^TNX", err)
	}
	if _, ok := quotes["KRW=X"]; !ok {
		t.Fatalf("partial KRW=X quote missing")
	}
	if err := transport.Verify(); err != nil {
		t.Fatal(err)
	}
}
