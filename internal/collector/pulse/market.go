package pulse

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/fifi/internal/external/yahoo"
	"golang.org/x/sync/errgroup"
)

// macroSymbols은 9개 심볼 및 라벨 정의.
var macroSymbols = []struct {
	symbol string
	label  string
}{
	{"^KS11", "KOSPI(야후분봉)"},
	{"^KQ11", "KOSDAQ(야후분봉)"},
	{"KRW=X", "원/달러"},
	{"NQ=F", "나스닥100선물"},
	{"ES=F", "S&P500선물"},
	{"YM=F", "다우선물"},
	{"^N225", "닛케이225"},
	{"CL=F", "WTI원유"},
	{"^TNX", "미국채10Y"},
}

// collectMarket은 Yahoo 분봉(1d/5m)으로 Window를 계산합니다.
// 심볼 9개를 병렬로 호출하고, 하나가 실패해도 나머지를 계속 처리합니다.
func collectMarket(ctx context.Context, yahooClient YahooQuotes, now time.Time, errors map[string]string) map[string]Window {
	symbols := make([]string, len(macroSymbols))
	for i, ms := range macroSymbols {
		symbols[i] = ms.symbol
	}

	// GetQuotes로 현재가 + 전일대비% 조회
	quotes, quoteErr := yahooClient.GetQuotes(ctx, symbols)
	if quoteErr != nil {
		// 부분 결과도 사용하되 에러 기록
		errors["yahoo_quotes"] = quoteErr.Error()
	}
	if quotes == nil {
		quotes = map[string]yahoo.Quote{}
	}

	// 5분봉 병렬 조회
	var mu sync.Mutex
	histories := make(map[string][]yahoo.DailyClose, len(symbols))

	g, gctx := errgroup.WithContext(ctx)
	for _, ms := range macroSymbols {
		sym := ms.symbol
		g.Go(func() error {
			hist, err := yahooClient.GetChartHistory(gctx, sym, "1d", "5m")
			if err != nil {
				mu.Lock()
				errors["yahoo_hist_"+sym] = fmt.Sprintf("5m hist: %v", err)
				mu.Unlock()
				return nil // 개별 실패 무시
			}
			mu.Lock()
			histories[sym] = hist
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait() // 에러는 errgroup 내부에서 errors 맵에 기록

	result := make(map[string]Window, len(macroSymbols))
	for _, ms := range macroSymbols {
		sym := ms.symbol
		quote := quotes[sym]
		hist := histories[sym]

		win := buildWindow(sym, ms.label, quote, hist, now)
		result[sym] = win
	}
	return result
}

func symbolVenue(sym string) string {
	switch sym {
	case "^KS11", "^KQ11":
		return "KRX"
	case "KRW=X":
		return "USD/KRW"
	case "NQ=F", "ES=F", "YM=F":
		return "CME"
	case "CL=F":
		return "NYMEX"
	case "^TNX":
		return "CBOE"
	case "^N225":
		return "JPX"
	default:
		return "OTHER"
	}
}

// buildWindow keeps the latest bar price and its timestamp together. Quotes are
// used only when no valid bar is available; daily change needs a provider baseline.
func buildWindow(symbol, label string, quote yahoo.Quote, series []yahoo.DailyClose, now time.Time) Window {
	w := Window{Symbol: symbol, Label: label, FetchedAt: now, Source: "Yahoo Finance " + symbol}
	if symbol == "KRW=X" {
		w.Source = "Yahoo Finance KRW=X spot (NDF/서울 현물 아님)"
	}
	bars := make([]yahoo.DailyClose, 0, len(series))
	for _, x := range series {
		if x.DateUnix > 0 && x.DateUnix <= now.Unix() && x.Close > 0 && finite(x.Close) {
			bars = append(bars, x)
		}
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].DateUnix < bars[j].DateUnix })
	if len(bars) > 0 {
		x := bars[len(bars)-1]
		w.Current = x.Close
		w.LastTS = time.Unix(x.DateUnix, 0)
	} else if finite(quote.Price) && quote.Price > 0 {
		w.Current = quote.Price
		if quote.MarketTimeUnix > 0 {
			w.LastTS = time.Unix(quote.MarketTimeUnix, 0)
		}
	} else {
		w.Reason = "데이터 없음"
		return w
	}
	w.OK = true
	// Quote baseline is usable only in the same provider observation date.
	if quote.PreviousClose > 0 && finite(quote.PreviousClose) && quote.MarketTimeUnix > 0 && time.Unix(quote.MarketTimeUnix, 0).UTC().Format("20060102") == w.LastTS.UTC().Format("20060102") {
		w.PrevClose = quote.PreviousClose
		w.ChangePct = (w.Current/w.PrevClose - 1) * 100
		w.ChangeOK = true
	}
	w.Freshness, w.AgeSeconds, w.StaleReason = DetermineFreshness(symbolVenue(symbol), w.LastTS, now, IsHoliday(symbolVenue(symbol), now.In(kstLocation).Format("20060102")))
	if !usableFreshness(w.Freshness) {
		w.Reason = w.StaleReason
		return w
	}
	for _, h := range []int{1, 2} {
		ref, partial := atOrBefore(bars, w.LastTS.Unix()-int64(h)*3600)
		if ref == nil || partial {
			w.Reason = "NO_VALID_WINDOW_ANCHOR (1h/2h 기준점 허용 오차 5분)"
			continue
		}
		rt := time.Unix(ref.DateUnix, 0)
		if (symbol == "^KS11" || symbol == "^KQ11" || symbol == "^N225") && !sameDay(rt, w.LastTS) {
			continue
		}
		v := (w.Current/ref.Close - 1) * 100
		if h == 1 {
			w.Move1hPct = &v
			w.Ref1hTS = &rt
		} else {
			w.Move2hPct = &v
			w.Ref2hTS = &rt
		}
	}
	return w
}

// A 5-minute series may anchor at most 5 minutes before the target.
func atOrBefore(series []yahoo.DailyClose, targetTS int64) (*yahoo.DailyClose, bool) {
	var best *yahoo.DailyClose
	for i := range series {
		x := series[i]
		if x.DateUnix <= targetTS && (best == nil || x.DateUnix > best.DateUnix) {
			best = &x
		}
	}
	if best == nil {
		return nil, true
	}
	return best, targetTS-best.DateUnix > 300
}
