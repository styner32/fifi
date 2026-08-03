package companyanalysis

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func (s *Service) buildAnnualRecords(facts *secCompanyFactsResponse) ([]AnnualRecord, float64, []string, error) {
	if facts == nil {
		return nil, 0, nil, fmt.Errorf("company facts are required")
	}

	revenueByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{
		"RevenueFromContractWithCustomerExcludingAssessedTax",
		"Revenues",
		"SalesRevenueNet",
	}, "USD", true)
	if len(revenueByFY) == 0 {
		return nil, 0, nil, fmt.Errorf("annual revenue series missing")
	}

	ebitByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"OperatingIncomeLoss"}, "USD", true)
	netIncomeByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"NetIncomeLoss"}, "USD", true)
	taxExpenseByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"IncomeTaxExpenseBenefit"}, "USD", true)
	effectiveTaxByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"EffectiveIncomeTaxRateContinuingOperations"}, "pure", true)
	dnaByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{
		"DepreciationDepletionAndAmortization",
		"DepreciationAndAmortization",
		"Depreciation",
	}, "USD", true)
	directCapExByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"PaymentsToAcquirePropertyPlantAndEquipment"}, "USD", true)
	currentAssetsByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"AssetsCurrent"}, "USD", false)
	currentLiabilitiesByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"LiabilitiesCurrent"}, "USD", false)
	totalAssetsByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"Assets"}, "USD", false)
	cashByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"CashAndCashEquivalentsAtCarryingValue"}, "USD", false)
	longTermDebtByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"LongTermDebt", "LongTermDebtNoncurrent"}, "USD", false)
	currentDebtByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"LongTermDebtCurrent", "DebtCurrent", "ConvertibleDebtCurrent"}, "USD", false)
	equityByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"StockholdersEquity"}, "USD", false)
	interestExpenseByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"InterestExpense", "InterestExpenseDebt", "InterestExpenseNonoperating"}, "USD", true)
	ppeByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"PropertyPlantAndEquipmentNet"}, "USD", false)
	sharesByFY := selectAnnualByFY(facts.Facts.DEI, []string{"EntityCommonStockSharesOutstanding"}, "shares", false)
	latestShares, sharesOK := selectLatestObservation(facts.Facts.DEI, []string{"EntityCommonStockSharesOutstanding"}, "shares", false)

	years := make([]int, 0, len(revenueByFY))
	for year := range revenueByFY {
		years = append(years, year)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	financials := make([]AnnualRecord, 0, len(years))
	for _, year := range years {
		revenueObs := revenueByFY[year]
		record := AnnualRecord{
			FiscalYear: year,
			EndDate:    revenueObs.End,
			FiledDate:  revenueObs.Filed,
			Revenue:    revenueObs.Val,
		}
		if obs, ok := ebitByFY[year]; ok {
			record.EBIT = obs.Val
		}
		if obs, ok := netIncomeByFY[year]; ok {
			record.NetIncome = obs.Val
		}
		if obs, ok := taxExpenseByFY[year]; ok {
			record.TaxExpense = obs.Val
		}
		if obs, ok := effectiveTaxByFY[year]; ok {
			record.EffectiveTax = obs.Val
		}
		if obs, ok := dnaByFY[year]; ok {
			record.DnA = math.Abs(obs.Val)
		}
		if obs, ok := directCapExByFY[year]; ok {
			record.CapEx = math.Abs(obs.Val)
		}
		if obs, ok := currentAssetsByFY[year]; ok {
			record.CurrentAssets = obs.Val
		}
		if obs, ok := currentLiabilitiesByFY[year]; ok {
			record.CurrentLiabilities = obs.Val
		}
		if obs, ok := totalAssetsByFY[year]; ok {
			record.TotalAssets = obs.Val
		}
		if obs, ok := cashByFY[year]; ok {
			record.Cash = obs.Val
		}
		if obs, ok := longTermDebtByFY[year]; ok {
			record.TotalDebt += obs.Val
		}
		if obs, ok := currentDebtByFY[year]; ok {
			record.TotalDebt += obs.Val
		}
		if obs, ok := equityByFY[year]; ok {
			record.Equity = obs.Val
		}
		if obs, ok := interestExpenseByFY[year]; ok {
			record.InterestExpense = math.Abs(obs.Val)
		}
		if obs, ok := ppeByFY[year]; ok {
			record.PropertyPlantEquipment = obs.Val
		}
		if obs, ok := sharesByFY[year]; ok {
			record.SharesOutstanding = obs.Val
		}
		if record.EffectiveTax == 0 {
			if derivedTax, ok := deriveEffectiveTax(record); ok {
				record.EffectiveTax = derivedTax
			}
		}
		financials = append(financials, record)
	}

	for i := 0; i < len(financials); i++ {
		if financials[i].CapEx == 0 && i+1 < len(financials) {
			deltaPPE := financials[i].PropertyPlantEquipment - financials[i+1].PropertyPlantEquipment
			capex := deltaPPE + financials[i].DnA
			if capex > 0 {
				financials[i].CapEx = capex
			}
		}
		if i+1 < len(financials) {
			currentNWC := financials[i].CurrentAssets - financials[i].CurrentLiabilities
			prevNWC := financials[i+1].CurrentAssets - financials[i+1].CurrentLiabilities
			financials[i].ChangeInNWC = currentNWC - prevNWC
		}
	}

	notes := make([]string, 0, 2)
	if !sharesOK {
		notes = append(notes, "latest SEC shares outstanding observation missing; annual FY figure used when available")
	}
	var latestShareValue float64
	if sharesOK {
		latestShareValue = latestShares.Val
	}
	return financials, latestShareValue, notes, nil
}

func selectAnnualByFY(concepts map[string]secConcept, tags []string, unit string, duration bool) map[int]secObservation {
	selected := make(map[int]rankedObservation)
	for priority, tag := range tags {
		concept, ok := concepts[tag]
		if !ok {
			continue
		}
		for _, observation := range concept.Units[unit] {
			if !isAnnualObservation(observation, duration) {
				continue
			}
			current, exists := selected[observation.FY]
			if !exists || priority < current.Priority || (priority == current.Priority && filedLater(observation, current.Observation)) {
				selected[observation.FY] = rankedObservation{Observation: observation, Priority: priority}
			}
		}
	}

	result := make(map[int]secObservation, len(selected))
	for year, ranked := range selected {
		result[year] = ranked.Observation
	}
	return result
}

func selectLatestObservation(concepts map[string]secConcept, tags []string, unit string, annualOnly bool) (secObservation, bool) {
	var best secObservation
	found := false
	for _, tag := range tags {
		concept, ok := concepts[tag]
		if !ok {
			continue
		}
		for _, observation := range concept.Units[unit] {
			if annualOnly && !isAnnualForm(observation.Form) {
				continue
			}
			if strings.TrimSpace(observation.End) == "" {
				continue
			}
			if !found || latestObservation(observation, best) {
				best = observation
				found = true
			}
		}
	}
	return best, found
}

func isAnnualObservation(observation secObservation, duration bool) bool {
	if observation.FY == 0 || !isAnnualForm(observation.Form) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(observation.FP), "FY") {
		return true
	}
	if !duration {
		return strings.TrimSpace(observation.End) != ""
	}
	if observation.Start == "" || observation.End == "" {
		return false
	}
	start, err := time.Parse("2006-01-02", observation.Start)
	if err != nil {
		return false
	}
	end, err := time.Parse("2006-01-02", observation.End)
	if err != nil {
		return false
	}
	days := end.Sub(start).Hours() / 24
	return days >= 300 && days <= 380
}

func isAnnualForm(form string) bool {
	switch strings.ToUpper(strings.TrimSpace(form)) {
	case "10-K", "10-K/A", "20-F", "20-F/A", "40-F", "40-F/A":
		return true
	default:
		return false
	}
}

func filedLater(left secObservation, right secObservation) bool {
	if left.Filed == right.Filed {
		return left.End > right.End
	}
	return left.Filed > right.Filed
}

func latestObservation(left secObservation, right secObservation) bool {
	if left.End == right.End {
		return left.Filed > right.Filed
	}
	return left.End > right.End
}

func deriveEffectiveTax(record AnnualRecord) (float64, bool) {
	preTaxIncome := record.NetIncome + record.TaxExpense
	if preTaxIncome <= 0 {
		return 0, false
	}
	tax := record.TaxExpense / preTaxIncome
	if tax < 0 {
		tax = 0
	}
	if tax > 1 {
		tax = 1
	}
	return tax, true
}
