package domesticstock

import "github.com/fifi/internal/dcf"

type DCFInputStatus string

const (
	DCFInputExact   DCFInputStatus = "exact"
	DCFInputDerived DCFInputStatus = "derived"
	DCFInputAssumed DCFInputStatus = "assumed"
	DCFInputMissing DCFInputStatus = "missing"
)

type DCFInputValue struct {
	Name     string
	Status   DCFInputStatus
	Value    float64
	HasValue bool
	Source   string
	Note     string
}

type DCFReadinessResult struct {
	Symbol                    string
	Division                  string
	CanProjectFCF             bool
	CanComputeWACC            bool
	CanComputeEnterpriseValue bool
	CanComputeTargetPrice     bool
	BalancePeriods            int
	IncomePeriods             int
	Inputs                    []DCFInputValue
	MissingForFCF             []string
	MissingForWACC            []string
	MissingForTargetPrice     []string
}

type DCFValuationOptions struct {
	DivClsCode       string
	ForecastYears    int
	TerminalGrowth   float64
	IndexCode        string
	BetaLookbackDays int
	RiskFreeRate     *float64
	RiskFreeBondCode string
	RiskFreeBondDiv  string
	Beta             *float64
	MarketPremium    *float64
	CostOfDebt       *float64
	NetDebt          *float64
}

type DCFValuationResult struct {
	Symbol       string
	Division     string
	CurrentPrice float64
	Inputs       []DCFInputValue
	Notes        []string
	Financial    dcf.FinancialData
	Market       dcf.MarketData
	Assumptions  dcf.Assumptions
	Projection   dcf.ProjectionModel
	Valuation    *dcf.ValuationResult
}

type dcfInputBundle struct {
	Symbol       string
	Division     string
	CurrentPrice float64
	Inputs       []DCFInputValue
	Notes        []string
	BalanceRows  []map[string]any
	IncomeRows   []map[string]any
	PriceRow     map[string]any
	Financial    dcf.FinancialData
	Market       dcf.MarketData
	Projection   dcf.ProjectionModel
	Assumptions  dcf.Assumptions
	MarketCap    float64
	HasMarketCap bool
}
