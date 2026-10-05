package pulse

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/parse"
)

type indexStock interface {
	InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error)
}

// collectIndex는 KIS inquire-index-price (TRID FHPUP02100000)로 지수 현황을 가져옵니다.
// indexCode: "0001" (KOSPI), "1001" (KOSDAQ)
func collectIndex(ctx context.Context, stock indexStock, indexCode string, now time.Time) (IndexLevel, error) {
	resp, err := stock.InquireIndexPrice(ctx, indexCode)
	if err != nil {
		return IndexLevel{}, fmt.Errorf("inquire-index-price (%s): %w", indexCode, err)
	}

	if resp == nil || !resp.IsOK() {
		return IndexLevel{}, fmt.Errorf("index %s: invalid business response", indexCode)
	}
	row := resp.FirstRow("output")
	if row == nil {
		return IndexLevel{}, fmt.Errorf("inquire-index-price (%s): output 행 없음", indexCode)
	}

	for _, key := range []string{"bstp_nmix_prpr", "bstp_nmix_prdy_vrss", "bstp_nmix_prdy_ctrt"} {
		if v, ok := parse.Num(row, key); !ok || !finite(v) {
			return IndexLevel{}, fmt.Errorf("index %s: missing/invalid %s", indexCode, key)
		}
	}
	var missing []string
	get := func(key string) float64 {
		v, ok := parse.Num(row, key)
		if !ok || !finite(v) {
			missing = append(missing, key)
			return 0
		}
		return v
	}
	getInt := func(key string) int {
		v, _ := parse.Int(row, key)
		return v
	}

	price := get("bstp_nmix_prpr")
	prdyVrss := get("bstp_nmix_prdy_vrss")
	if sign := stringField(row, "prdy_vrss_sign"); sign == "4" || sign == "5" {
		prdyVrss = -math.Abs(prdyVrss)
	}
	prevClose := price - prdyVrss
	if price <= 0 || prevClose <= 0 {
		return IndexLevel{}, fmt.Errorf("index %s: nonpositive price", indexCode)
	}
	changePct := 0.0
	if prevClose != 0 {
		changePct = prdyVrss / prevClose * 100
	}

	// 전일 대비 % 필드가 있으면 우선 사용
	if v, ok := parse.Num(row, "bstp_nmix_prdy_ctrt"); ok {
		changePct = v
	}

	tradingValue := get("acml_tr_pbmn") / millionToEok // 백만원 → 억원

	upperLimit := getInt("uplm_issu_cnt")
	advancers := getInt("ascn_issu_cnt")
	unchanged := getInt("stnr_issu_cnt")
	decliners := getInt("down_issu_cnt")
	lowerLimit := getInt("lslm_issu_cnt")
	totalCount := upperLimit + advancers + unchanged + decliners + lowerLimit

	lastTS := sourceTimestamp(row)
	freshness, ageSecs, staleReason := DetermineFreshness("KRX", lastTS, now, false)

	level := IndexLevel{
		Price:        price,
		PrevClose:    prevClose,
		ChangePct:    changePct,
		Open:         get("bstp_nmix_oprc"),
		High:         get("bstp_nmix_hgpr"),
		Low:          get("bstp_nmix_lwpr"),
		TradingValue: tradingValue,
		UpperLimit:   upperLimit,
		Advancers:    advancers,
		Unchanged:    unchanged,
		Decliners:    decliners,
		LowerLimit:   lowerLimit,
		TotalCount:   totalCount,
		OK:           true, BreadthOK: validFields(row, "ascn_issu_cnt", "down_issu_cnt", "stnr_issu_cnt", "uplm_issu_cnt", "lslm_issu_cnt"), Source: "KIS inquire-index-price",
		LastTS:      lastTS,
		FetchedAt:   now,
		Freshness:   freshness,
		AgeSeconds:  ageSecs,
		StaleReason: staleReason,
	}
	level.MissingFields = missing
	return level, nil
}
