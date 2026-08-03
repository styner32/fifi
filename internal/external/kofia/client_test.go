package kofia

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/fifi/internal/testhelpers"
)

func TestClientGetMarketFunds(t *testing.T) {
	transport := testhelpers.NewMockTransport()
	transport.New(baseURL).
		Post("/meta/getMetaDataList.do").
		Reply(http.StatusOK).
		BodyString(`{
			"unit": "천원",
			"ds1": [
				{
					"TMPV1": "20260703",
					"TMPV2": 1000000000,
					"TMPV3": 200000000,
					"TMPV4": 3000000000,
					"TMPV5": 40000000,
					"TMPV6": 5000000,
					"TMPV7": 12.5
				}
			]
		}`)

	client := NewClient("test-ua")
	client.httpClient.Transport = transport

	rows, err := client.GetMarketFunds(context.Background(), "20260703", "20260703")
	if err != nil {
		t.Fatalf("GetMarketFunds error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row.Date != "20260703" {
		t.Errorf("expected Date 20260703, got %s", row.Date)
	}
	if row.CustomerDepositMln != 1000000.0 {
		t.Errorf("expected CustomerDepositMln 1000000, got %v", row.CustomerDepositMln)
	}
	if row.ForcedSellRatioPct != 12.5 {
		t.Errorf("expected ForcedSellRatioPct 12.5, got %v", row.ForcedSellRatioPct)
	}

	if err := transport.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestCachedClientGetMarketFundsForDate(t *testing.T) {
	tmpDir := t.TempDir()

	transport := testhelpers.NewMockTransport()
	transport.New(baseURL).
		Post("/meta/getMetaDataList.do").
		Reply(http.StatusOK).
		BodyString(`{
			"unit": "천원",
			"ds1": [
				{
					"TMPV1": "20260703",
					"TMPV2": 1000000000,
					"TMPV3": 200000000,
					"TMPV4": 3000000000,
					"TMPV5": 40000000,
					"TMPV6": 5000000,
					"TMPV7": 12.5
				}
			]
		}`)

	cc := NewCachedClient(tmpDir, "test-ua")
	cc.client.httpClient.Transport = transport

	// First load (cache miss, fetches from mock server)
	row1, err := cc.GetMarketFundsForDate(context.Background(), "20260703")
	if err != nil {
		t.Fatalf("GetMarketFundsForDate miss error: %v", err)
	}
	if row1.CustomerDepositMln != 1000000.0 {
		t.Errorf("expected CustomerDepositMln 1000000, got %v", row1.CustomerDepositMln)
	}

	// Verify transport call
	if err := transport.Verify(); err != nil {
		t.Fatal(err)
	}

	// Second load (cache hit, no request)
	transport.Reset()
	row2, err := cc.GetMarketFundsForDate(context.Background(), "20260703")
	if err != nil {
		t.Fatalf("GetMarketFundsForDate hit error: %v", err)
	}
	if row2.CustomerDepositMln != 1000000.0 {
		t.Errorf("expected CustomerDepositMln 1000000, got %v", row2.CustomerDepositMln)
	}
	if len(transport.Requests()) != 0 {
		t.Errorf("expected 0 requests on cache hit, got %d", len(transport.Requests()))
	}

	// Cache file exists
	cachePath := filepath.Join(tmpDir, "kofia_market_funds.20260703.json")
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("cache file not found: %v", err)
	}
}
