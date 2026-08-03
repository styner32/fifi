package dcf

import (
	"fmt"
	"strings"
)

type FinancialData struct {
	Revenue      float64 `json:"revenue"`
	EBIT         float64 `json:"ebit"`
	EffectiveTax float64 `json:"effective_tax"`
	DnA          float64 `json:"dna"`
	CapEx        float64 `json:"capex"`
	ChangeInNWC  float64 `json:"change_in_nwc"`
	SharesOut    float64 `json:"shares_out"`
	NetDebt      float64 `json:"net_debt"`
}

type MarketData struct {
	RiskFreeRate  float64 `json:"risk_free_rate"`
	Beta          float64 `json:"beta"`
	MarketPremium float64 `json:"market_premium"`
	CostOfDebt    float64 `json:"cost_of_debt"`
	EquityWeight  float64 `json:"equity_weight"`
	DebtWeight    float64 `json:"debt_weight"`
}

type Assumptions struct {
	ForecastYears    int     `json:"forecast_years"`
	TerminalGrowth   float64 `json:"terminal_growth"`
	TargetPriceScale float64 `json:"target_price_scale,omitempty"`
	TargetPriceUnit  string  `json:"target_price_unit,omitempty"`
}

type ProjectionModel struct {
	RevenueGrowth float64 `json:"revenue_growth"`
	EBITMargin    float64 `json:"ebit_margin"`
	DNAMargin     float64 `json:"dna_margin"`
	CapExMargin   float64 `json:"capex_margin"`
	NWCMargin     float64 `json:"nwc_margin"`
}

type ForecastYear struct {
	Year           int     `json:"year"`
	Revenue        float64 `json:"revenue"`
	EBIT           float64 `json:"ebit"`
	DnA            float64 `json:"dna"`
	CapEx          float64 `json:"capex"`
	ChangeInNWC    float64 `json:"change_in_nwc"`
	FCF            float64 `json:"fcf"`
	DiscountFactor float64 `json:"discount_factor"`
	PresentValue   float64 `json:"present_value"`
}

type ValuationResult struct {
	BaseFCF              float64         `json:"base_fcf"`
	CostOfEquity         float64         `json:"cost_of_equity"`
	WACC                 float64         `json:"wacc"`
	TerminalValue        float64         `json:"terminal_value"`
	TerminalPresentValue float64         `json:"terminal_present_value"`
	EnterpriseValue      float64         `json:"enterprise_value"`
	EquityValue          float64         `json:"equity_value"`
	TargetPriceRaw       float64         `json:"target_price_raw"`
	TargetPriceScale     float64         `json:"target_price_scale"`
	TargetPriceUnit      string          `json:"target_price_unit"`
	TargetPrice          float64         `json:"target_price"`
	Forecast             []ForecastYear  `json:"forecast"`
	Projection           ProjectionModel `json:"projection"`
	Assumptions          Assumptions     `json:"assumptions"`
}

const (
	defaultTargetPriceScale = 100_000_000.0
	defaultTargetPriceUnit  = "KRW/share"
)

func FCF(fin FinancialData) float64 {
	return (fin.EBIT * (1 - fin.EffectiveTax)) + fin.DnA - fin.CapEx - fin.ChangeInNWC
}

func CostOfEquity(market MarketData) float64 {
	return market.RiskFreeRate + (market.Beta * market.MarketPremium)
}

func WACC(fin FinancialData, market MarketData) float64 {
	ke := CostOfEquity(market)
	return (market.EquityWeight * ke) + (market.DebtWeight * market.CostOfDebt * (1 - fin.EffectiveTax))
}

func Value(fin FinancialData, market MarketData, assumptions Assumptions, model ProjectionModel) (*ValuationResult, error) {
	if fin.SharesOut <= 0 {
		return nil, fmt.Errorf("shares out must be positive")
	}

	assumptions = normalizeAssumptions(assumptions)
	model = normalizeProjectionModel(model)
	wacc := WACC(fin, market)
	forecast, err := buildForecast(fin, assumptions, model)
	if err != nil {
		return nil, err
	}
	return valueForecast(fin, assumptions, model, forecast, wacc, CostOfEquity(market))
}

func normalizeProjectionModel(model ProjectionModel) ProjectionModel {
	model.RevenueGrowth = clamp(model.RevenueGrowth, -0.20, 0.25)
	model.EBITMargin = clamp(model.EBITMargin, -0.20, 0.60)
	model.DNAMargin = clamp(model.DNAMargin, 0, 0.30)
	model.CapExMargin = clamp(model.CapExMargin, 0, 0.40)
	model.NWCMargin = clamp(model.NWCMargin, -0.20, 0.20)
	return model
}

func normalizeAssumptions(assumptions Assumptions) Assumptions {
	if assumptions.TargetPriceScale == 0 {
		assumptions.TargetPriceScale = defaultTargetPriceScale
	}
	if strings.TrimSpace(assumptions.TargetPriceUnit) == "" {
		assumptions.TargetPriceUnit = defaultTargetPriceUnit
	}
	return assumptions
}

func clamp(value float64, lower float64, upper float64) float64 {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}
