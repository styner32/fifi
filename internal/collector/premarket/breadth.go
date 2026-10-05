package premarket

import (
	"context"
	"fmt"
	"github.com/fifi/internal/external/dataapi"
	"strconv"
	"strings"
	"time"
)

type DailyPrices interface {
	DailyStockPrices(context.Context, string) ([]dataapi.DailyStockRow, error)
}

func collectBreadth(ctx context.Context, client DailyPrices, prior string, now time.Time, out map[string]Observation, errs map[string]string) {
	if client == nil || prior == "" {
		return
	}
	rows, err := client.DailyStockPrices(ctx, prior)
	if err != nil {
		errs["breadth"] = err.Error()
		return
	}
	counts, err := countBreadth(rows, prior)
	if err != nil {
		errs["breadth"] = err.Error()
		return
	}
	for _, market := range []string{"KOSPI", "KOSDAQ"} {
		c := counts[market]
		total := c[0] + c[1] + c[2]
		if total == 0 {
			errs[strings.ToLower(market)+"_breadth"] = "no stocks in complete daily cross-section"
			continue
		}
		for i, field := range []string{"advancers", "decliners", "unchanged", "advancing_pct"} {
			unit, v := "종목", 0.0
			if i < 3 {
				v = float64(c[i])
			} else {
				unit = "%"
				v = float64(c[0]) * 100 / float64(total)
			}
			out[strings.ToLower(market)+"_"+field] = Observation{Value: ptr(v), Unit: unit, BusinessDate: prior, Source: "금융위원회 주식시세정보 (data.go.kr)", Session: "DAILY_BREADTH", Status: "VALID", FetchedAt: now, SampleCount: total, Reason: "제공사 시장별 주식 전체; 우선주 및 거래정지 포함, 지수 구성종목 수와 다름; 등락 0은 보합"}
		}
	}
}
func countBreadth(rows []dataapi.DailyStockRow, prior string) (map[string][3]int, error) {
	counts := map[string][3]int{}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.Date != prior || r.ISIN == "" || seen[r.ISIN] {
			return nil, fmt.Errorf("breadth date/identity/duplicate validation failed")
		}
		seen[r.ISIN] = true
		if r.Market != "KOSPI" && r.Market != "KOSDAQ" && r.Market != "KONEX" {
			return nil, fmt.Errorf("unknown breadth market")
		}
		change, err := strconv.ParseFloat(strings.ReplaceAll(r.Change, ",", ""), 64)
		if err != nil || !finite(change) {
			return nil, fmt.Errorf("invalid stock change in breadth")
		}
		close, err := strconv.ParseFloat(strings.ReplaceAll(r.Close, ",", ""), 64)
		if err != nil || !finite(close) || close <= 0 {
			return nil, fmt.Errorf("invalid stock close in breadth")
		}
		count := counts[r.Market]
		switch {
		case change > 0:
			count[0]++
		case change < 0:
			count[1]++
		default:
			count[2]++
		}
		counts[r.Market] = count
	}
	return counts, nil
}
