package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/fifi/internal/external/kofia"
)

// CreditSection은 mktfunds API + KOFIA FreeSIS에서 추출한 시장 전체 신용잔고 데이터입니다.
// mktfunds API 원시값은 억원 단위입니다 (KOFIA freesis 기준 검증 완료).
// KOFIA FreeSIS 원시값은 백만원 단위입니다.
type CreditSection struct {
	KofiaAmountUnit             string        `json:"kofia_amount_unit"`
	MissingFields               []string      `json:"missing_fields,omitempty"`
	DepositReconciliationStatus QualityStatus `json:"deposit_reconciliation_status"`
	CreditLoanBalanceEok        float64       `json:"credit_loan_balance_eok"` // 신용융자 잔고 (억원) — KIS mktfunds
	CustomerDepositEok          float64       `json:"customer_deposit_eok"`    // 고객예탁금 (억원) — KIS mktfunds
	DepositChangeEok            float64       `json:"deposit_change_eok"`      // 예탁금 전일 대비 (억원) — KIS mktfunds
	FuturesDepositEok           float64       `json:"futures_deposit_eok"`     // 선물예수금 (억원) — KIS mktfunds

	// KOFIA FreeSIS 반대매매 데이터 (백만원 → 억원 변환: 원자료 / 100)
	MarginReceivableEok      float64 `json:"margin_receivable_eok,omitempty"`       // 위탁매매 미수금 (억원)
	MarginReceivablePrevEok  float64 `json:"margin_receivable_prev_eok,omitempty"`  // 전일 위탁매매 미수금 (억원)
	MarginReceivableDeltaEok float64 `json:"margin_receivable_delta_eok,omitempty"` // 미수금 전일 대비 (억원)

	ForcedSellAmountEok     float64 `json:"forced_sell_amount_eok,omitempty"`      // 실제 반대매매금액 (억원)
	ForcedSellAmountPrevEok float64 `json:"forced_sell_amount_prev_eok,omitempty"` // 전일 실제 반대매매금액 (억원)
	ForcedSellDeltaEok      float64 `json:"forced_sell_delta_eok,omitempty"`       // 반대매매 전일 대비 (억원)
	ForcedSellRatioPct      float64 `json:"forced_sell_ratio_pct,omitempty"`       // 전일 미수금 대비 반대매매비중(%)
	ForcedSellStatus        string  `json:"forced_sell_status,omitempty"`          // BELOW_CUSTOM_ALERT_THRESHOLD

	RawForcedSellMln       float64       `json:"raw_forced_sell_mln,omitempty"`
	RawMarginReceivableMln float64       `json:"raw_margin_receivable_mln,omitempty"`
	Date                   string        `json:"date,omitempty"`           // 영업일 (mktfunds)
	KofiaDate              string        `json:"kofia_date,omitempty"`     // KOFIA 데이터 영업일
	ReferenceDate          string        `json:"reference_date,omitempty"` // 후행 통계 기준일자 (YYYY-MM-DD)
	MarketSessionDate      string        `json:"market_session_date,omitempty"`
	FreshnessStatus        string        `json:"freshness_status,omitempty"`
	LagDays                int           `json:"lag_days,omitempty"`
	Status                 QualityStatus `json:"status,omitempty"`
	Reason                 string        `json:"reason,omitempty"`
}

const kofiaMillionToEok = 100.0

func collectCredit(ctx context.Context, stock DomesticStock, kofiaClient KOFIAClient, date string) (*CreditSection, error) {
	if stock == nil {
		return nil, fmt.Errorf("domestic stock dependency is nil")
	}
	resp, err := stock.MarketFunds(ctx, date)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("market funds business error")
	}
	var row, prev map[string]any
	for _, r := range rows(resp, "output") {
		d := sourceDate(r)
		if d != "" && d <= date && (row == nil || d > sourceDate(row)) {
			row = r
		}
	}
	if row != nil {
		for _, r := range rows(resp, "output") {
			d := sourceDate(r)
			if d != "" && d < sourceDate(row) && (prev == nil || d > sourceDate(prev)) {
				prev = r
			}
		}
	}
	if row == nil {
		return nil, fmt.Errorf("mktfunds output missing")
	}

	credit, creditOK := num(row, "crdt_loan_rmnd")
	deposit, depositOK := num(row, "cust_dpmn_amt")
	depositChange, changeOK := num(row, "cust_dpmn_amt_prdy_vrss")
	futures, futuresOK := num(row, "futs_tfam_amt")
	if !creditOK || !depositOK || credit <= 0 || deposit <= 0 {
		return nil, fmt.Errorf("credit/deposit missing or invalid")
	}
	bsopDate, _ := row["bsop_date"].(string)

	// mktfunds API 원시값은 이미 억원 단위 — 변환 불필요.
	section := &CreditSection{
		CreditLoanBalanceEok: credit,
		CustomerDepositEok:   deposit,
		DepositChangeEok:     depositChange,
		FuturesDepositEok:    futures,
		Date:                 bsopDate,
		Status:               StatusLaggedContextOnly,
		FreshnessStatus:      string(StatusLaggedContextOnly),
		LagDays:              0,
		MarketSessionDate:    date,
	}

	if !changeOK {
		section.MissingFields = append(section.MissingFields, "deposit_change_eok")
	}
	if !futuresOK {
		section.MissingFields = append(section.MissingFields, "futures_deposit_eok")
	}
	reference, _ := time.Parse("20060102", bsopDate)
	target, _ := time.Parse("20060102", date)
	section.LagDays = int(target.Sub(reference).Hours() / 24)
	section.ReferenceDate = bsopDate
	section.DepositReconciliationStatus = StatusNotEvaluated
	if prev != nil && changeOK {
		if pv, ok := num(prev, "cust_dpmn_amt"); ok {
			section.DepositReconciliationStatus, _ = ValidateCredit(deposit, pv, depositChange)
		}
	}
	// KOFIA 반대매매 데이터 추가 (실패해도 section은 반환)
	if kofiaClient != nil {
		kofiaRow, err := kofiaClient.GetMarketFundsForDate(ctx, date)
		if err != nil {
			section.Reason = fmt.Sprintf("kofia: %v", err)
		} else if kofiaRow != nil && kofiaRow.AmountUnit == "KRW_MILLION" && validDate(kofiaRow.Date) && kofiaRow.Date <= date && finite(kofiaRow.MarginReceivableMln) && finite(kofiaRow.ForcedSellAmountMln) && finite(kofiaRow.ForcedSellRatioPct) && kofiaRow.MarginReceivableMln > 0 && kofiaRow.ForcedSellAmountMln >= 0 && kofiaRow.ForcedSellRatioPct >= 0 {
			section.RawMarginReceivableMln = kofiaRow.MarginReceivableMln
			section.RawForcedSellMln = kofiaRow.ForcedSellAmountMln

			// FreeSIS MarginReceivableMln & ForcedSellAmountMln are in 백만원 (Million KRW)
			// Strictly convert 백만원 to 억원 by dividing by 100.0
			marginEok := kofiaRow.MarginReceivableMln / kofiaMillionToEok
			forcedEok := kofiaRow.ForcedSellAmountMln / kofiaMillionToEok

			section.MarginReceivableEok = marginEok
			section.ForcedSellAmountEok = forcedEok
			section.ForcedSellRatioPct = kofiaRow.ForcedSellRatioPct
			section.ForcedSellStatus = "SOURCE_REPORTED_RATIO / DENOMINATOR_NOT_RECONCILED"
			section.KofiaDate = kofiaRow.Date
			section.KofiaAmountUnit = kofiaRow.AmountUnit

		}
	}
	if section.KofiaDate == "" && section.Reason == "" {
		section.Reason = "KOFIA_UNIT_DATE_OR_VALUE_UNVERIFIED"
	}
	return section, nil
}

// KOFIAClient is the interface for KOFIA FreeSIS data access.
type KOFIAClient interface {
	GetMarketFundsForDate(ctx context.Context, date string) (*kofia.MarketFundsRow, error)
}
