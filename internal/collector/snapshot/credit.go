package snapshot

import (
	"context"
	"fmt"

	"github.com/fifi/internal/external/kofia"
)

// CreditSection은 mktfunds API + KOFIA FreeSIS에서 추출한 시장 전체 신용잔고 데이터입니다.
// mktfunds API 원시값은 억원 단위입니다 (KOFIA freesis 기준 검증 완료).
// KOFIA FreeSIS 원시값은 백만원 단위입니다.
type CreditSection struct {
	CreditLoanBalanceEok float64 `json:"credit_loan_balance_eok"` // 신용융자 잔고 (억원) — KIS mktfunds
	CustomerDepositEok   float64 `json:"customer_deposit_eok"`    // 고객예탁금 (억원) — KIS mktfunds
	DepositChangeEok     float64 `json:"deposit_change_eok"`      // 예탁금 전일 대비 (억원) — KIS mktfunds
	FuturesDepositEok    float64 `json:"futures_deposit_eok"`     // 선물예수금 (억원) — KIS mktfunds
	// KOFIA FreeSIS 반대매매 데이터 (백만원 → 억원 변환)
	MarginReceivableEok float64 `json:"margin_receivable_eok,omitempty"` // 위탁매매 미수금 (억원)
	ForcedSellAmountEok float64 `json:"forced_sell_amount_eok,omitempty"` // 실제 반대매매금액 (억원)
	ForcedSellRatioPct  float64 `json:"forced_sell_ratio_pct,omitempty"`  // 미수금 대비 반대매매비중(%)
	Date                string  `json:"date,omitempty"`                   // 영업일
	KofiaDate           string  `json:"kofia_date,omitempty"`             // KOFIA 데이터 영업일
	Reason              string  `json:"-"`
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
	row := firstRow(resp, "output")
	if row == nil {
		return nil, fmt.Errorf("mktfunds output missing")
	}

	credit, _ := num(row, "crdt_loan_rmnd")
	deposit, _ := num(row, "cust_dpmn_amt")
	depositChange, _ := num(row, "cust_dpmn_amt_prdy_vrss")
	futures, _ := num(row, "futs_tfam_amt")
	bsopDate, _ := row["bsop_date"].(string)

	// mktfunds API 원시값은 이미 억원 단위 — 변환 불필요.
	section := &CreditSection{
		CreditLoanBalanceEok: credit,
		CustomerDepositEok:   deposit,
		DepositChangeEok:     depositChange,
		FuturesDepositEok:    futures,
		Date:                 bsopDate,
	}

	// KOFIA 반대매매 데이터 추가 (실패해도 section은 반환)
	if kofiaClient != nil {
		kofiaRow, err := kofiaClient.GetMarketFundsForDate(ctx, date)
		if err != nil {
			section.Reason = fmt.Sprintf("kofia: %v", err)
		} else if kofiaRow != nil {
			section.MarginReceivableEok = kofiaRow.MarginReceivableMln / kofiaMillionToEok
			section.ForcedSellAmountEok = kofiaRow.ForcedSellAmountMln / kofiaMillionToEok
			section.ForcedSellRatioPct = kofiaRow.ForcedSellRatioPct
			section.KofiaDate = kofiaRow.Date
		}
	}

	return section, nil
}

// KOFIAClient is the interface for KOFIA FreeSIS data access.
type KOFIAClient interface {
	GetMarketFundsForDate(ctx context.Context, date string) (*kofia.MarketFundsRow, error)
}
