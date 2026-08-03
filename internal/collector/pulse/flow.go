package pulse

import (
	"context"
	"fmt"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/parse"
)

type flowStock interface {
	InquireInvestorTimeByMarket(context.Context, string, string) (*auth.RESTResponse, error)
}

// collectFlow는 KIS inquire-investor-time-by-market (TRID FHPTJ04030000)로
// 현재까지 누적된 수급 스냅샷을 가져옵니다.
// marketDiv: "KSP" (KOSPI) 또는 "KSQ" (KOSDAQ)
// indexDiv : "0001" (KOSPI) 또는 "1001" (KOSDAQ)
func collectFlow(ctx context.Context, stock flowStock, marketDiv, indexDiv string, now time.Time) (FlowSnapshot, error) {
	resp, err := stock.InquireInvestorTimeByMarket(ctx, marketDiv, indexDiv)
	if err != nil {
		return FlowSnapshot{}, fmt.Errorf("inquire-investor-time-by-market (%s): %w", marketDiv, err)
	}

	row := resp.FirstRow("output")
	if row == nil {
		return FlowSnapshot{}, fmt.Errorf("inquire-investor-time-by-market (%s): output 행 없음", marketDiv)
	}

	get := func(key string) float64 {
		v, _ := parse.Num(row, key)
		return v / millionToEok // 백만원 → 억원
	}

	return FlowSnapshot{
		Foreign:     get("frgn_ntby_tr_pbmn"),
		Institution: get("orgn_ntby_tr_pbmn"),
		Individual:  get("prsn_ntby_tr_pbmn"),
		FinInvest:   get("scrt_ntby_tr_pbmn"),
		InvTrust:    get("ivtr_ntby_tr_pbmn"),
		Pension:     get("fund_ntby_tr_pbmn"),
		PrivEquity:  get("pe_fund_ntby_tr_pbmn"),
		Insurance:   get("insu_ntby_tr_pbmn"),
		Bank:        get("bank_ntby_tr_pbmn"),
		EtcCorp:     get("etc_corp_ntby_tr_pbmn"),
		EtcForeign:  get("etc_frgn_ntby_tr_pbmn"),
		EtcFin:      get("mrbn_ntby_tr_pbmn"),
		OK:          true,
	}, nil
}
