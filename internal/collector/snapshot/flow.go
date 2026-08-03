package snapshot

import (
	"context"
	"fmt"
)

const tradeAmountMillionToEok = 100.0

type FlowSection struct {
	Date           string
	ForeignEok     float64
	InstitutionEok float64
	IndividualEok  float64
}

// collectFlow converts KIS *_tr_pbmn investor net trade amounts from million
// KRW to eok KRW by dividing by 100.
func collectFlow(ctx context.Context, stock DomesticStock, date string) (*FlowSection, error) {
	if stock == nil {
		return nil, fmt.Errorf("domestic stock dependency is nil")
	}
	resp, err := stock.InquireInvestorDailyByMarket(ctx, date)
	if err != nil {
		return nil, err
	}
	row := firstRow(resp, "output")
	if row == nil {
		return nil, fmt.Errorf("investor daily output missing")
	}
	foreign, ok := num(row, "frgn_ntby_tr_pbmn")
	if !ok {
		return nil, fmt.Errorf("frgn_ntby_tr_pbmn missing")
	}
	institution, ok := num(row, "orgn_ntby_tr_pbmn")
	if !ok {
		return nil, fmt.Errorf("orgn_ntby_tr_pbmn missing")
	}
	individual, ok := num(row, "prsn_ntby_tr_pbmn")
	if !ok {
		return nil, fmt.Errorf("prsn_ntby_tr_pbmn missing")
	}
	return &FlowSection{
		Date:           date,
		ForeignEok:     foreign / tradeAmountMillionToEok,
		InstitutionEok: institution / tradeAmountMillionToEok,
		IndividualEok:  individual / tradeAmountMillionToEok,
	}, nil
}
