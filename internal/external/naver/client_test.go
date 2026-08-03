package naver

import (
	"context"
	"net/http"
	"testing"

	"github.com/fifi/internal/testhelpers"
)

func TestGetIndexQuote(t *testing.T) {
	transport := testhelpers.NewMockTransport()
	transport.New(pollingBaseURL).
		Get("/api/realtime?query=SERVICE_INDEX:VKOSPI").
		Reply(http.StatusOK).
		BodyString(`{
			"resultCode": "success",
			"result": {
				"areas": [
					{
						"datas": [
							{
								"cd": "VKOSPI",
								"nv": 1550,
								"cv": 50,
								"cr": 3.33,
								"ov": 1500,
								"hv": 1600,
								"lv": 1490,
								"ms": "OPEN"
							}
						]
					}
				]
			}
		}`)

	client := NewClient(&http.Client{Transport: transport}, "test-ua")
	quote, err := client.GetIndexQuote(context.Background(), "VKOSPI")
	if err != nil {
		t.Fatalf("GetIndexQuote error: %v", err)
	}

	if quote.Price != 15.5 {
		t.Errorf("expected Price 15.5, got %v", quote.Price)
	}
	if quote.Change != 0.5 {
		t.Errorf("expected Change 0.5, got %v", quote.Change)
	}
	if quote.ChangePercent != 3.33 {
		t.Errorf("expected ChangePercent 3.33, got %v", quote.ChangePercent)
	}
	if quote.MarketStatus != "OPEN" {
		t.Errorf("expected MarketStatus OPEN, got %v", quote.MarketStatus)
	}

	if err := transport.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestGetIndexDailyHistory(t *testing.T) {
	transport := testhelpers.NewMockTransport()
	transport.New(siseBaseURL).
		Get("/sise/sise_index_day.naver?code=KOSPI&page=1").
		Reply(http.StatusOK).
		BodyString(`
			<table>
				<tr class="item">
					<td class="date">2026.07.03</td>
					<td class="number_1">2,700.50</td>
				</tr>
				<tr class="item">
					<td class="date">2026.07.02</td>
					<td class="number_1">2,690.20</td>
				</tr>
			</table>
		`)

	client := NewClient(&http.Client{Transport: transport}, "test-ua")
	history, err := client.GetIndexDailyHistory(context.Background(), "KOSPI", 2)
	if err != nil {
		t.Fatalf("GetIndexDailyHistory error: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	// ascending order
	if history[0].Date != "2026.07.02" || history[0].Close != 2690.2 {
		t.Errorf("first history wrong: %+v", history[0])
	}
	if history[1].Date != "2026.07.03" || history[1].Close != 2700.5 {
		t.Errorf("second history wrong: %+v", history[1])
	}
}
