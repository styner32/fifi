package companyanalysis

import (
	"github.com/fifi/internal/dcf"
	"github.com/fifi/internal/fileio"
)

type Export struct {
	GeneratedAt   string                `json:"generated_at"`
	BusinessDate  string                `json:"business_date,omitempty"`
	Symbol        string                `json:"symbol"`
	Result        *Result               `json:"result,omitempty"`
	ReverseDCF    *dcf.ReverseDCFResult `json:"reverse_dcf,omitempty"`
	MonteCarloCfg dcf.MonteCarloConfig  `json:"monte_carlo_config"`
	MonteCarlo    *dcf.MonteCarloResult `json:"monte_carlo,omitempty"`
}

func WriteExport(path string, payload Export) error {
	return fileio.WriteJSONAtomic(path, payload)
}
