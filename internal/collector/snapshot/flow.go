package snapshot

import (
	"context"
	"fmt"
)

const tradeAmountMillionToEok = 100.0

type FlowSection struct {
	Date                             string   `json:"date"`
	ForeignEok                       float64  `json:"foreign_eok"`
	InstitutionEok                   float64  `json:"institution_eok"`
	IndividualEok                    float64  `json:"individual_eok"`
	ResidualEok                      float64  `json:"residual_eok"`
	DisplayedParticipantResidualEok float64  `json:"displayed_participant_residual_eok"`
	MissingParticipants             bool     `json:"missing_participants"`
	ReconciliationStatus             string   `json:"reconciliation_status"`
	ForeignPrevEok                   *float64 `json:"foreign_prev_eok,omitempty"`
	InstitutionPrevEok               *float64 `json:"institution_prev_eok,omitempty"`
	IndividualPrevEok                *float64 `json:"individual_prev_eok,omitempty"`
	PreviousFlowStatus               string   `json:"previous_flow_status,omitempty"`
	PreviousFlowReason               string   `json:"previous_flow_reason,omitempty"`
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
	fEok := foreign / tradeAmountMillionToEok
	iEok := institution / tradeAmountMillionToEok
	pEok := individual / tradeAmountMillionToEok
	residual := fEok + iEok + pEok

	status := "RECONCILED"
	missingParticipants := false
	if residual != 0 {
		status = string(StatusPartiallyReconciled)
		missingParticipants = true
	}

	return &FlowSection{
		Date:                             date,
		ForeignEok:                       fEok,
		InstitutionEok:                   iEok,
		IndividualEok:                    pEok,
		ResidualEok:                      residual,
		DisplayedParticipantResidualEok: residual,
		MissingParticipants:             missingParticipants,
		ReconciliationStatus:             status,
		PreviousFlowStatus:               string(StatusMissing),
		PreviousFlowReason:               "PREVIOUS_DAY_FLOW_NOT_COLLECTED",
	}, nil
}
