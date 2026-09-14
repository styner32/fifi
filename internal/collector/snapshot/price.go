package snapshot

import (
	"context"
	"fmt"
)

type PriceSection struct {
	Date                        string  `json:"date"`
	Open                        float64 `json:"open"`
	High                        float64 `json:"high"`
	Low                         float64 `json:"low"`
	Close                       float64 `json:"close"`
	PreviousClose               float64 `json:"previous_close"`
	RangePoints                 float64 `json:"range_points"`
	RangePercent                float64 `json:"range_percent"`
	IntradayRangePoints         float64 `json:"intraday_range_points"`
	IntradayRangePctOfPrevClose float64 `json:"intraday_range_pct_of_prev_close"`
	ClosePositionPct            float64 `json:"close_position_pct"`
	PriceRegime                 string  `json:"price_regime"`
	YearHigh                    bool    `json:"year_high"`
	TradingValueEok             float64 `json:"trading_value_eok"` // 일간 총 거래대금 (억원), 0 = 미확인
}

// collectPrice computes intraday range as high-low and range percent as
// (high-low)/previous close*100.
//
// Primary: InquireIndexDailyPrice에서 date 매칭 row 사용.
// Fallback 1: date 매칭 실패 시 dailyRows의 가장 최근 row 사용 (날짜 라벨링은 해당 row의 실제 날짜).
// Fallback 2: today에 한해 InquireIndexPrice 실시간 조회 (staleness 감지 포함).
func collectPrice(ctx context.Context, stock DomesticStock, date string) (*PriceSection, error) {
	if stock == nil {
		return nil, fmt.Errorf("domestic stock dependency is nil")
	}
	dailyRows, err := stock.InquireIndexDailyPrice(ctx, "0001", date)
	if err == nil {
		// 1차: 정확한 날짜 매칭
		if section, ok := priceFromRows(dailyRows, date); ok {
			return section, nil
		}
		// 1-1차: 가장 최근 row 사용 (rows[0]이 최신, 내림차순)
		if len(dailyRows) > 0 {
			actualDate := fmt.Sprintf("%v", dailyRows[0]["stck_bsop_date"])
			if section, ok := priceFromRow(dailyRows[0], actualDate); ok {
				fmt.Printf("[price] Warning: date %s not found in daily rows, using latest available row %s\n", date, actualDate)
				return section, nil
			}
		}
	}
	today, _ := normalizeDate("")
	if date != today {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("KOSPI daily row not found for %s", date)
	}
	resp, fallbackErr := stock.InquireIndexPrice(ctx, "0001")
	if fallbackErr != nil {
		if err != nil {
			return nil, fmt.Errorf("%v; fallback: %w", err, fallbackErr)
		}
		return nil, fallbackErr
	}
	row := firstRow(resp, "output")
	if section, ok := priceFromRow(row, date); ok {
		// Staleness 감지: close와 prevClose가 동일하면 장 시작 전 데이터일 가능성
		if section.Close > 0 && section.PreviousClose > 0 && section.Close == section.PreviousClose {
			fmt.Printf("[price] Warning: close (%.2f) == previousClose (%.2f) for %s — market may not have opened yet, data could be stale\n",
				section.Close, section.PreviousClose, date)
		}
		return section, nil
	}
	return nil, fmt.Errorf("KOSPI index price output missing")
}

func priceFromRows(dailyRows []map[string]any, date string) (*PriceSection, bool) {
	for _, row := range dailyRows {
		if fmt.Sprintf("%v", row["stck_bsop_date"]) == date {
			return priceFromRow(row, date)
		}
	}
	return nil, false
}

func priceFromRow(row map[string]any, date string) (*PriceSection, bool) {
	closeValue, closeOK := num(row, "bstp_nmix_prpr", "stck_clpr", "stck_prpr")
	openValue, openOK := num(row, "bstp_nmix_oprc", "stck_oprc")
	highValue, highOK := num(row, "bstp_nmix_hgpr", "stck_hgpr")
	lowValue, lowOK := num(row, "bstp_nmix_lwpr", "stck_lwpr")
	if !closeOK || !openOK || !highOK || !lowOK {
		return nil, false
	}
	prevClose, ok := num(row, "stck_prdy_clpr", "bstp_nmix_prdy_clpr")
	if !ok {
		if diff, diffOK := num(row, "bstp_nmix_prdy_vrss", "prdy_vrss"); diffOK {
			prevClose = closeValue - diff
		}
	}
	if prevClose <= 0 {
		return nil, false
	}
	yearHigh, _ := num(row, "dryy_bstp_nmix_hgpr")
	rangePoints := highValue - lowValue
	rangePctOfPrevClose := rangePoints / prevClose * 100

	closePosPct := 0.0
	if rangePoints > 0 {
		closePosPct = (closeValue - lowValue) / rangePoints * 100
	}

	priceRegime := "KOSPI_STRONG_UP"
	if closeValue > prevClose {
		if closePosPct < 80.0 {
			priceRegime = "KOSPI_STRONG_UP_OFF_HIGH"
		}
	} else {
		priceRegime = "KOSPI_WEAK"
	}

	// acml_tr_pbmn: 누적거래대금 (백만원), divide by 100 → 억원
	tradingMillionKRW, _ := num(row, "acml_tr_pbmn")
	return &PriceSection{
		Date: date, Open: openValue, High: highValue, Low: lowValue,
		Close: closeValue, PreviousClose: prevClose, RangePoints: rangePoints,
		RangePercent: rangePctOfPrevClose, IntradayRangePoints: rangePoints,
		IntradayRangePctOfPrevClose: rangePctOfPrevClose, ClosePositionPct: closePosPct,
		PriceRegime: priceRegime, YearHigh: yearHigh > 0 && highValue >= yearHigh,
		TradingValueEok: tradingMillionKRW / tradeAmountMillionToEok,
	}, true
}
