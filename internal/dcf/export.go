package dcf

import (
	"github.com/fifi/internal/fileio"
)

type MonteCarloExport struct {
	GeneratedAt   string            `json:"generated_at"`
	BusinessDate  string            `json:"business_date,omitempty"`
	Symbol        string            `json:"symbol"`
	CurrentPrice  float64           `json:"current_price"`
	Financial     FinancialData     `json:"financial"`
	Market        MarketData        `json:"market"`
	Assumptions   Assumptions       `json:"assumptions"`
	Projection    ProjectionModel   `json:"projection"`
	Valuation     *ValuationResult  `json:"valuation,omitempty"`
	ReverseDCF    *ReverseDCFResult `json:"reverse_dcf,omitempty"`
	MonteCarloCfg MonteCarloConfig  `json:"monte_carlo_config"`
	MonteCarlo    *MonteCarloResult `json:"monte_carlo,omitempty"`
}

func WriteMonteCarloExport(path string, payload MonteCarloExport) error {
	return fileio.WriteJSONAtomic(path, payload)
}
