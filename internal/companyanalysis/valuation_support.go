package companyanalysis

import (
	"math"

	"github.com/fifi/internal/dcf"
)

func deriveProjectionModel(financials []AnnualRecord) dcf.ProjectionModel {
	model := dcf.ProjectionModel{
		RevenueGrowth: 0.05,
		EBITMargin:    0.10,
		DNAMargin:     0.03,
		CapExMargin:   0.04,
		NWCMargin:     0.01,
	}

	if growth, ok := deriveRevenueGrowth(financials); ok {
		model.RevenueGrowth = growth
	}
	if margin, ok := averageMargin(financials, func(record AnnualRecord) float64 { return record.EBIT }); ok {
		model.EBITMargin = margin
	}
	if margin, ok := averageMargin(financials, func(record AnnualRecord) float64 { return record.DnA }); ok {
		model.DNAMargin = math.Max(margin, 0)
	}
	if margin, ok := averageMargin(financials, func(record AnnualRecord) float64 { return record.CapEx }); ok {
		model.CapExMargin = math.Max(margin, 0)
	}
	if margin, ok := averageMargin(financials, func(record AnnualRecord) float64 { return record.ChangeInNWC }); ok {
		model.NWCMargin = margin
	}
	return model
}

func deriveRevenueGrowth(financials []AnnualRecord) (float64, bool) {
	values := make([]float64, 0, len(financials))
	for _, record := range financials {
		if record.Revenue <= 0 {
			continue
		}
		values = append(values, record.Revenue)
		if len(values) >= 5 {
			break
		}
	}
	if len(values) < 2 {
		return 0, false
	}
	latest := values[0]
	oldest := values[len(values)-1]
	years := float64(len(values) - 1)
	if oldest <= 0 || years <= 0 {
		return 0, false
	}
	growth := math.Pow(latest/oldest, 1/years) - 1
	if math.IsNaN(growth) || math.IsInf(growth, 0) {
		return 0, false
	}
	return growth, true
}

func averageMargin(financials []AnnualRecord, numerator func(AnnualRecord) float64) (float64, bool) {
	var sum float64
	var count int
	for _, record := range financials {
		if record.Revenue <= 0 {
			continue
		}
		sum += numerator(record) / record.Revenue
		count++
		if count >= 4 {
			break
		}
	}
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

func buildKeyMetrics(quote Quote, latest AnnualRecord, marketCap float64, netDebt float64, revenueGrowth float64) KeyMetrics {
	enterpriseValue := marketCap + latest.TotalDebt - latest.Cash
	currentRatio := 0.0
	if latest.CurrentLiabilities > 0 {
		currentRatio = latest.CurrentAssets / latest.CurrentLiabilities
	}
	debtToEquity := 0.0
	if latest.Equity > 0 {
		debtToEquity = latest.TotalDebt / latest.Equity
	}
	cashToDebt := 0.0
	if latest.TotalDebt > 0 {
		cashToDebt = latest.Cash / latest.TotalDebt
	}
	operatingMargin := 0.0
	netMargin := 0.0
	roe := 0.0
	if latest.Revenue > 0 {
		operatingMargin = latest.EBIT / latest.Revenue
		netMargin = latest.NetIncome / latest.Revenue
	}
	if latest.Equity > 0 {
		roe = latest.NetIncome / latest.Equity
	}
	return KeyMetrics{
		MarketCap:       marketCap,
		EnterpriseValue: enterpriseValue,
		NetDebt:         netDebt,
		RevenueGrowth:   revenueGrowth,
		OperatingMargin: operatingMargin,
		NetMargin:       netMargin,
		ROE:             roe,
		CurrentRatio:    currentRatio,
		DebtToEquity:    debtToEquity,
		CashToDebt:      cashToDebt,
	}
}

func inputStatusForValue(value float64) InputStatus {
	if value == 0 {
		return InputMissing
	}
	return InputExact
}

func averageFloatSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
