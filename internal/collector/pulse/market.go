package pulse

import (
	"context"
	"fmt"
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

// buildWindow는 분봉 시리즈 + 현재가로 Window를 계산합니다.
// §6.1 알고리즘: 절대 unix 타임스탬프 기준 at-or-before 선택.
func buildWindow(symbol, label string, quote yahoo.Quote, series []yahoo.DailyClose, now time.Time) Window {
	win := Window{Symbol: symbol, Label: label}

	if len(series) == 0 && quote.Price == 0 {
		win.Reason = "데이터 없음"
		return win
	}

	win.OK = true
	win.ChangePct = quote.ChangePercent

	// 현재가: quote.Price 우선, 없으면 시리즈 마지막 점
	current := quote.Price
	if current == 0 && len(series) > 0 {
		current = series[len(series)-1].Close
	}
	win.Current = current

	// 앵커: 시리즈의 마지막 타임스탬프
	if len(series) > 0 {
		lastItem := series[len(series)-1]
		win.LastTS = time.Unix(lastItem.DateUnix, 0)
	} else if quote.MarketTimeUnix > 0 {
		win.LastTS = time.Unix(quote.MarketTimeUnix, 0)
	}

	win.FetchedAt = now
	venue := symbolVenue(symbol)
	nowKST := now.In(kstLocation)
	isHoliday := IsHoliday(venue, nowKST.Format("20060102"))

	freshness, ageSecs, staleReason := DetermineFreshness(venue, win.LastTS, now, isHoliday)
	win.Freshness = freshness
	win.AgeSeconds = ageSecs
	if staleReason != "" {
		win.StaleReason = staleReason
	}

	if win.Freshness == "HOLIDAY" || win.Freshness == "STALE" {
		win.Move1hPct = nil
		win.Move2hPct = nil
		if win.Freshness == "HOLIDAY" {
			win.Reason = fmt.Sprintf("%s %s 휴장 · 1h/2h N/A · 신호점수 제외", nowKST.Format("2006-01-02"), label)
		} else {
			win.Reason = win.StaleReason
		}
		return win
	}

	if len(series) == 0 {
		win.Reason = "분봉 없음 (현재가만 사용)"
		return win
	}

	lastTS := series[len(series)-1].DateUnix

	for _, delta := range []struct {
		hours    int
		ptrField **float64
	}{
		{1, &win.Move1hPct},
		{2, &win.Move2hPct},
	} {
		targetTS := lastTS - int64(delta.hours)*3600
		ref, partial := atOrBefore(series, targetTS)
		if ref == nil {
			continue // 데이터 부족
		}
		if ref.Close == 0 {
			continue
		}
		movePct := (current - ref.Close) / ref.Close * 100
		*delta.ptrField = ptr(movePct)

		// KRW=X 희소 시리즈: 간격이 45분 이상이면 근사 표기
		if symbol == "KRW=X" && partial {
			win.Reason = "KRW=X 희소 시리즈 — 인접 과거점 근사 사용"
		}
		if partial && win.Reason == "" {
			win.Reason = "부분 데이터 (룩백이 시리즈 시작보다 과거)"
		}
	}

	return win
}

// atOrBefore는 series에서 targetTS ≤ ts 조건 중 ts 최대인 점을 반환합니다.
// 없으면 첫 점을 반환하고 partial=true.
func atOrBefore(series []yahoo.DailyClose, targetTS int64) (ref *yahoo.DailyClose, partial bool) {
	// 시리즈는 오름차순 정렬됨
	best := -1
	for i, item := range series {
		if item.DateUnix <= targetTS {
			best = i
		}
	}
	if best >= 0 {
		item := series[best]
		// 45분(2700초) 이상 차이면 희소
		gap := targetTS - item.DateUnix
		return &item, gap > 2700
	}
	// 타깃이 시리즈 시작보다 과거 → 첫 점 fallback
	if len(series) > 0 {
		item := series[0]
		return &item, true
	}
	return nil, false
}
