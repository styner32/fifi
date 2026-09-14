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

	row := resp.FirstRow("output")
	if row == nil {
		return IndexLevel{}, fmt.Errorf("inquire-index-price (%s): output 행 없음", indexCode)
	}

	get := func(key string) float64 {
		v, _ := parse.Num(row, key)
		return v
	}
	getInt := func(key string) int {
		v, _ := parse.Int(row, key)
		return v
	}

	price := get("bstp_nmix_prpr")
	prdyVrss := get("bstp_nmix_prdy_vrss")
	prevClose := price - prdyVrss
	changePct := 0.0
	if prevClose != 0 {
		changePct = prdyVrss / prevClose * 100
	}

	// 전일 대비 % 필드가 있으면 우선 사용
	if v, ok := parse.Num(row, "bstp_nmix_prdy_ctrt"); ok && math.Abs(v) > 0.0001 {
		changePct = v
	}

	tradingValue := get("acml_tr_pbmn") / millionToEok // 백만원 → 억원

	upperLimit := getInt("uplm_issu_cnt")
	advancers := getInt("ascn_issu_cnt")
	unchanged := getInt("stnr_issu_cnt")
	decliners := getInt("down_issu_cnt")
	lowerLimit := getInt("lslm_issu_cnt")
	totalCount := upperLimit + advancers + unchanged + decliners + lowerLimit

	// Time / Freshness calculation
	nowKST := now.In(kstLocation)
	kst330 := time.Date(nowKST.Year(), nowKST.Month(), nowKST.Day(), 15, 30, 0, 0, kstLocation)
	var lastTS time.Time
	if nowKST.After(kst330) {
		lastTS = kst330
	} else {
		lastTS = now
	}

	freshness, ageSecs, staleReason := DetermineFreshness("KRX", lastTS, now, false)

	return IndexLevel{
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
		OK:           true,
		LastTS:       lastTS,
		FetchedAt:    now,
		Freshness:    freshness,
		AgeSeconds:   ageSecs,
		StaleReason:  staleReason,
	}, nil
}
