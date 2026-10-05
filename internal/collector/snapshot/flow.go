package snapshot

import (
	"context"
	"fmt"
)

const tradeAmountMillionToEok = 100.0

type FlowSection struct {
	OtherCorporateEok               *float64 `json:"other_corporate_eok"`
	AllParticipantResidualEok       *float64 `json:"all_participant_residual_eok"`
	PreviousDate                    string   `json:"previous_date,omitempty"`
	Status                          string   `json:"status"`
	Date                            string   `json:"date"`
	ForeignEok                      float64  `json:"foreign_eok"`
	InstitutionEok                  float64  `json:"institution_eok"`
	IndividualEok                   float64  `json:"individual_eok"`
	ResidualEok                     float64  `json:"residual_eok"`
	DisplayedParticipantResidualEok float64  `json:"displayed_participant_residual_eok"`
	MissingParticipants             bool     `json:"missing_participants"`
	ReconciliationStatus            string   `json:"reconciliation_status"`
	ForeignPrevEok                  *float64 `json:"foreign_prev_eok,omitempty"`
	InstitutionPrevEok              *float64 `json:"institution_prev_eok,omitempty"`
	IndividualPrevEok               *float64 `json:"individual_prev_eok,omitempty"`
	PreviousFlowStatus              string   `json:"previous_flow_status,omitempty"`
	PreviousFlowReason              string   `json:"previous_flow_reason,omitempty"`
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
	if !resp.IsOK() {
		return nil, fmt.Errorf("investor daily business error")
	}
	var row, previous map[string]any
	for _, r := range rows(resp, "output") {
		d := sourceDate(r)
		if d == date {
			if row != nil {
				return nil, fmt.Errorf("duplicate flow date")
			}
			row = r
		}
		if d != "" && d < date && (previous == nil || d > sourceDate(previous)) {
			previous = r
		}
	}
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

	status := "PARTICIPANT_SCOPE_UNVERIFIED"
	missingParticipants := true
	if residual != 0 {
		status = string(StatusPartiallyReconciled)
		missingParticipants = true
	}

	section := &FlowSection{
		Date:                            date,
		ForeignEok:                      fEok,
		InstitutionEok:                  iEok,
		IndividualEok:                   pEok,
		ResidualEok:                     residual,
		DisplayedParticipantResidualEok: residual,
		MissingParticipants:             missingParticipants,
		ReconciliationStatus:            status,
		PreviousFlowStatus:              string(StatusMissing),
		PreviousFlowReason:              "PREVIOUS_DAY_FLOW_NOT_COLLECTED",
	}
	section.Status = "DATED_DAILY_OBSERVATION"
	if other, ok := num(row, "etc_corp_ntby_tr_pbmn"); ok {
		section.OtherCorporateEok = ptr(other / 100)
		section.AllParticipantResidualEok = ptr(residual + other/100)
		section.ReconciliationStatus = "PARTIALLY_RECONCILED"
	}
	if previous != nil {
		f, fo := num(previous, "frgn_ntby_tr_pbmn")
		i, io := num(previous, "orgn_ntby_tr_pbmn")
		p, po := num(previous, "prsn_ntby_tr_pbmn")
		if fo && io && po {
			section.ForeignPrevEok = ptr(f / 100)
			section.InstitutionPrevEok = ptr(i / 100)
			section.IndividualPrevEok = ptr(p / 100)
			section.PreviousDate = sourceDate(previous)
			section.PreviousFlowStatus = "SAME_RESPONSE_PREVIOUS_SESSION"
			section.PreviousFlowReason = ""
		}
	}
	return section, nil
}
