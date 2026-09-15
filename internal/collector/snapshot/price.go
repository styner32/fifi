package snapshot

import (
	"context"
	"fmt"
	"math"
)

type PriceSection struct {
	Status                      string   `json:"status"`
	MissingFields               []string `json:"missing_fields,omitempty"`
	Date                        string   `json:"date"`
	Open                        float64  `json:"open"`
	High                        float64  `json:"high"`
	Low                         float64  `json:"low"`
	Close                       float64  `json:"close"`
	PreviousClose               float64  `json:"previous_close"`
	RangePoints                 float64  `json:"range_points"`
	RangePercent                float64  `json:"range_percent"`
	IntradayRangePoints         float64  `json:"intraday_range_points"`
	IntradayRangePctOfPrevClose float64  `json:"intraday_range_pct_of_prev_close"`
	ClosePositionPct            float64  `json:"close_position_pct"`
	PriceRegime                 string   `json:"price_regime"`
	YearHigh                    bool     `json:"year_high"`
	TradingValueEok             float64  `json:"trading_value_eok"` // 일간 총 거래대금 (억원), 0 = 미확인
}

// collectPrice computes intraday range as high-low and range percent as
// (high-low)/previous close*100.
//
// Only a dated daily row may represent the requested business date.
func collectPrice(ctx context.Context, stock DomesticStock, date string) (*PriceSection, error) {
	if stock == nil {
		return nil, fmt.Errorf("domestic stock dependency is nil")
	}
	dailyRows, err := stock.InquireIndexDailyPrice(ctx, "0001", date)
	if err != nil {
		return nil, err
	}
	if section, ok := priceFromRows(dailyRows, date); ok {
		section.Status = "DATED_DAILY_OBSERVATION"
		return section, nil
	}
	return nil, fmt.Errorf("valid KOSPI daily row missing for %s; other dates are not substituted", date)
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
	if !closeOK || !openOK || !highOK || !lowOK || lowValue <= 0 || highValue < lowValue || openValue < lowValue || openValue > highValue || closeValue < lowValue || closeValue > highValue {
		return nil, false
	}
	prevClose, ok := num(row, "stck_prdy_clpr", "bstp_nmix_prdy_clpr")
	if !ok {
		if diff, diffOK := num(row, "bstp_nmix_prdy_vrss", "prdy_vrss"); diffOK {
			if sign := fmt.Sprint(row["prdy_vrss_sign"]); sign == "4" || sign == "5" {
				diff = -math.Abs(diff)
			}
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

	priceRegime := "KOSPI_UP"
	if closeValue > prevClose {
		if closePosPct < 80.0 {
			priceRegime = "KOSPI_UP_OFF_HIGH"
		}
	} else if closeValue == prevClose {
		priceRegime = "KOSPI_FLAT"
	} else {
		priceRegime = "KOSPI_DOWN"
	}

	// acml_tr_pbmn: 누적거래대금 (백만원), divide by 100 → 억원
	tradingMillionKRW, tvOK := num(row, "acml_tr_pbmn")
	var missing []string
	if !tvOK || tradingMillionKRW < 0 {
		missing = append(missing, "trading_value_eok")
		tradingMillionKRW = 0
	}
	if yearHigh <= 0 {
		missing = append(missing, "year_high")
	}
	if rangePoints == 0 {
		missing = append(missing, "close_position_pct")
	}
	return &PriceSection{
		MissingFields: missing, Date: date, Open: openValue, High: highValue, Low: lowValue,
		Close: closeValue, PreviousClose: prevClose, RangePoints: rangePoints,
		RangePercent: rangePctOfPrevClose, IntradayRangePoints: rangePoints,
		IntradayRangePctOfPrevClose: rangePctOfPrevClose, ClosePositionPct: closePosPct,
		PriceRegime: priceRegime, YearHigh: yearHigh > 0 && highValue >= yearHigh,
		TradingValueEok: tradingMillionKRW / tradeAmountMillionToEok,
	}, true
}
