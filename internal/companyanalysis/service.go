package companyanalysis

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fifi/internal/dcf"
)

const (
	defaultSECTickersURL       = "https://www.sec.gov/files/company_tickers.json"
	defaultSECCompanyFactsBase = "https://data.sec.gov/api/xbrl/companyfacts"
	defaultFRED10YearURL       = "https://fred.stlouisfed.org/graph/fredgraph.csv?id=DGS10"
	defaultStooqBaseURL        = "https://stooq.com/q/d/l/"
	defaultSECUserAgent        = "open-trading-api/1.0 contact@example.com"
	defaultBenchmarkSymbol     = "SPY"
	defaultForecastYears       = 5
	defaultTerminalGrowth      = 0.025
	defaultRiskFreeRate        = 0.04
	defaultMarketPremium       = 0.055
	defaultBetaLookbackDays    = 252
	defaultCostOfDebtSpread    = 0.015
	defaultPriceScale          = 1.0
	defaultPriceUnit           = "USD/share"
	minimumBetaObservations    = 60
	stooqInterval              = "d"
)

type InputStatus string

const (
	InputExact   InputStatus = "exact"
	InputDerived InputStatus = "derived"
	InputAssumed InputStatus = "assumed"
	InputMissing InputStatus = "missing"
)

type InputValue struct {
	Name     string      `json:"name"`
	Status   InputStatus `json:"status"`
	Value    float64     `json:"value,omitempty"`
	HasValue bool        `json:"has_value"`
	Source   string      `json:"source,omitempty"`
	Note     string      `json:"note,omitempty"`
}

type Config struct {
	SECTickersURL            string
	SECCompanyFactsBase      string
	FRED10YearURL            string
	StooqBaseURL             string
	SECUserAgent             string
	SECTickersCachePath      string
	SECCompanyFactsCachePath string
}

type AnalysisOptions struct {
	BenchmarkSymbol  string
	ForecastYears    int
	TerminalGrowth   float64
	PriceScale       float64
	PriceUnit        string
	BetaLookbackDays int
	RiskFreeRate     *float64
	Beta             *float64
	MarketPremium    *float64
	CostOfDebt       *float64
	NetDebt          *float64
}

type Quote struct {
	Symbol        string  `json:"symbol"`
	PriceDate     string  `json:"price_date"`
	Price         float64 `json:"price"`
	PreviousClose float64 `json:"previous_close"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	Currency      string  `json:"currency"`
	Source        string  `json:"source"`
}

type AnnualRecord struct {
	FiscalYear             int     `json:"fiscal_year"`
	EndDate                string  `json:"end_date"`
	FiledDate              string  `json:"filed_date,omitempty"`
	Revenue                float64 `json:"revenue,omitempty"`
	EBIT                   float64 `json:"ebit,omitempty"`
	NetIncome              float64 `json:"net_income,omitempty"`
	TaxExpense             float64 `json:"tax_expense,omitempty"`
	EffectiveTax           float64 `json:"effective_tax,omitempty"`
	DnA                    float64 `json:"dna,omitempty"`
	CapEx                  float64 `json:"capex,omitempty"`
	ChangeInNWC            float64 `json:"change_in_nwc,omitempty"`
	CurrentAssets          float64 `json:"current_assets,omitempty"`
	CurrentLiabilities     float64 `json:"current_liabilities,omitempty"`
	TotalAssets            float64 `json:"total_assets,omitempty"`
	TotalDebt              float64 `json:"total_debt,omitempty"`
	Cash                   float64 `json:"cash,omitempty"`
	Equity                 float64 `json:"equity,omitempty"`
	SharesOutstanding      float64 `json:"shares_outstanding,omitempty"`
	InterestExpense        float64 `json:"interest_expense,omitempty"`
	PropertyPlantEquipment float64 `json:"property_plant_equipment,omitempty"`
}

type KeyMetrics struct {
	MarketCap       float64 `json:"market_cap"`
	EnterpriseValue float64 `json:"enterprise_value"`
	NetDebt         float64 `json:"net_debt"`
	RevenueGrowth   float64 `json:"revenue_growth"`
	OperatingMargin float64 `json:"operating_margin"`
	NetMargin       float64 `json:"net_margin"`
	ROE             float64 `json:"roe"`
	CurrentRatio    float64 `json:"current_ratio"`
	DebtToEquity    float64 `json:"debt_to_equity"`
	CashToDebt      float64 `json:"cash_to_debt"`
}

type Result struct {
	Symbol          string               `json:"symbol"`
	CompanyName     string               `json:"company_name"`
	CIK             string               `json:"cik"`
	BenchmarkSymbol string               `json:"benchmark_symbol"`
	Quote           Quote                `json:"quote"`
	Inputs          []InputValue         `json:"inputs"`
	Notes           []string             `json:"notes,omitempty"`
	Financials      []AnnualRecord       `json:"financials"`
	KeyMetrics      KeyMetrics           `json:"key_metrics"`
	Financial       dcf.FinancialData    `json:"financial"`
	Market          dcf.MarketData       `json:"market"`
	Assumptions     dcf.Assumptions      `json:"assumptions"`
	Projection      dcf.ProjectionModel  `json:"projection"`
	Valuation       *dcf.ValuationResult `json:"valuation,omitempty"`
}

type Service struct {
	httpClient *http.Client
	cfg        Config
}

type secTickerEntry struct {
	CIK    int    `json:"cik_str"`
	Ticker string `json:"ticker"`
	Title  string `json:"title"`
}

type secCompanyFactsResponse struct {
	CIK        int    `json:"cik"`
	EntityName string `json:"entityName"`
	Facts      struct {
		DEI    map[string]secConcept `json:"dei"`
		USGAAP map[string]secConcept `json:"us-gaap"`
	} `json:"facts"`
}

type secConcept struct {
	Units map[string][]secObservation `json:"units"`
}

type secObservation struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Val   float64 `json:"val"`
	Accn  string  `json:"accn"`
	FY    int     `json:"fy"`
	FP    string  `json:"fp"`
	Form  string  `json:"form"`
	Filed string  `json:"filed"`
	Frame string  `json:"frame"`
}

type stooqRow struct {
	Date  string
	Close float64
}

type rankedObservation struct {
	Observation secObservation
	Priority    int
}

func NewService(httpClient *http.Client, cfg Config) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(cfg.SECTickersURL) == "" {
		cfg.SECTickersURL = defaultSECTickersURL
	}
	if strings.TrimSpace(cfg.SECCompanyFactsBase) == "" {
		cfg.SECCompanyFactsBase = defaultSECCompanyFactsBase
	}
	if strings.TrimSpace(cfg.FRED10YearURL) == "" {
		cfg.FRED10YearURL = defaultFRED10YearURL
	}
	if strings.TrimSpace(cfg.StooqBaseURL) == "" {
		cfg.StooqBaseURL = defaultStooqBaseURL
	}
	if strings.TrimSpace(cfg.SECUserAgent) == "" {
		cfg.SECUserAgent = defaultSECUserAgent
	}
	return &Service{httpClient: httpClient, cfg: cfg}
}

func (s *Service) Analyze(ctx context.Context, symbol string, options AnalysisOptions) (*Result, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	options = resolveOptions(options)

	stockHistory, err := s.fetchStooqHistory(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("stock history failed: %w", err)
	}
	quote, err := buildQuoteFromHistory(symbol, stockHistory)
	if err != nil {
		return nil, err
	}

	cik, companyName, err := s.resolveSECTicker(ctx, symbol)
	if err != nil {
		return nil, err
	}

	facts, err := s.fetchCompanyFacts(ctx, cik)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(companyName) == "" {
		companyName = strings.TrimSpace(facts.EntityName)
	}

	financials, latestShares, notes, err := s.buildAnnualRecords(facts)
	if err != nil {
		return nil, err
	}
	if len(financials) < 2 {
		return nil, fmt.Errorf("not enough annual financial history for %s", symbol)
	}
	latest := financials[0]

	inputs := make([]InputValue, 0, 16)
	if latest.Revenue <= 0 || latest.EBIT == 0 {
		return nil, fmt.Errorf("required SEC financial metrics missing for %s", symbol)
	}

	fin := dcf.FinancialData{
		Revenue: latest.Revenue,
		EBIT:    latest.EBIT,
		DnA:     latest.DnA,
		CapEx:   latest.CapEx,
	}
	inputs = append(inputs,
		InputValue{Name: "Revenue", Status: InputExact, Value: latest.Revenue, HasValue: true, Source: "sec.companyfacts"},
		InputValue{Name: "EBIT", Status: InputExact, Value: latest.EBIT, HasValue: true, Source: "sec.companyfacts"},
		InputValue{Name: "DnA", Status: inputStatusForValue(latest.DnA), Value: latest.DnA, HasValue: latest.DnA != 0, Source: "sec.companyfacts"},
		InputValue{Name: "CapEx", Status: inputStatusForValue(latest.CapEx), Value: latest.CapEx, HasValue: latest.CapEx != 0, Source: "sec.companyfacts / derived from PPE"},
	)

	if latest.EffectiveTax > 0 {
		fin.EffectiveTax = latest.EffectiveTax
		inputs = append(inputs, InputValue{Name: "EffectiveTax", Status: InputExact, Value: latest.EffectiveTax, HasValue: true, Source: "sec.companyfacts"})
	} else {
		fin.EffectiveTax = 0.21
		inputs = append(inputs, InputValue{Name: "EffectiveTax", Status: InputAssumed, Value: fin.EffectiveTax, HasValue: true, Source: "default", Note: "fallback assumption"})
	}

	if latest.ChangeInNWC != 0 {
		fin.ChangeInNWC = latest.ChangeInNWC
		inputs = append(inputs, InputValue{Name: "ChangeInNWC", Status: InputDerived, Value: latest.ChangeInNWC, HasValue: true, Source: "sec.companyfacts current assets/current liabilities"})
	} else {
		inputs = append(inputs, InputValue{Name: "ChangeInNWC", Status: InputMissing, Source: "sec.companyfacts", Note: "could not derive from annual balance-sheet history"})
	}

	if latestShares > 0 {
		fin.SharesOut = latestShares
		inputs = append(inputs, InputValue{Name: "SharesOut", Status: InputExact, Value: latestShares, HasValue: true, Source: "sec.companyfacts dei"})
	} else if latest.SharesOutstanding > 0 {
		fin.SharesOut = latest.SharesOutstanding
		inputs = append(inputs, InputValue{Name: "SharesOut", Status: InputExact, Value: latest.SharesOutstanding, HasValue: true, Source: "sec.companyfacts dei"})
	} else {
		return nil, fmt.Errorf("shares outstanding missing for %s", symbol)
	}

	netDebt := latest.TotalDebt - latest.Cash
	if options.NetDebt != nil {
		netDebt = *options.NetDebt
		inputs = append(inputs, InputValue{Name: "NetDebt", Status: InputAssumed, Value: netDebt, HasValue: true, Source: "env override"})
	} else {
		inputs = append(inputs, InputValue{Name: "NetDebt", Status: InputDerived, Value: netDebt, HasValue: true, Source: "sec.companyfacts debt - cash"})
	}
	fin.NetDebt = netDebt

	market := dcf.MarketData{}
	if options.RiskFreeRate != nil {
		market.RiskFreeRate = *options.RiskFreeRate
		inputs = append(inputs, InputValue{Name: "RiskFreeRate", Status: InputAssumed, Value: market.RiskFreeRate, HasValue: true, Source: "env override"})
	} else if value, ok := s.fetchRiskFreeRate(ctx); ok {
		market.RiskFreeRate = value
		inputs = append(inputs, InputValue{Name: "RiskFreeRate", Status: InputExact, Value: value, HasValue: true, Source: "fred.DGS10"})
	} else {
		market.RiskFreeRate = defaultRiskFreeRate
		inputs = append(inputs, InputValue{Name: "RiskFreeRate", Status: InputAssumed, Value: market.RiskFreeRate, HasValue: true, Source: "default", Note: "fallback assumption"})
		notes = append(notes, "risk free rate fallback used because FRED fetch failed")
	}

	if options.MarketPremium != nil {
		market.MarketPremium = *options.MarketPremium
		inputs = append(inputs, InputValue{Name: "MarketPremium", Status: InputAssumed, Value: market.MarketPremium, HasValue: true, Source: "env override"})
	} else if value, note, ok := s.resolveMarketPremium(ctx); ok {
		market.MarketPremium = value
		inputs = append(inputs, InputValue{Name: "MarketPremium", Status: InputExact, Value: value, HasValue: true, Source: "external-market-premium.damodaran", Note: note})
	} else {
		market.MarketPremium = defaultMarketPremium
		inputs = append(inputs, InputValue{Name: "MarketPremium", Status: InputAssumed, Value: market.MarketPremium, HasValue: true, Source: "default", Note: "fallback assumption"})
		notes = append(notes, "market premium fallback used because Damodaran provider was unavailable")
	}

	if options.Beta != nil {
		market.Beta = *options.Beta
		inputs = append(inputs, InputValue{Name: "Beta", Status: InputAssumed, Value: market.Beta, HasValue: true, Source: "env override"})
	} else {
		benchmarkHistory, benchErr := s.fetchStooqHistory(ctx, options.BenchmarkSymbol)
		if benchErr != nil {
			market.Beta = 1.0
			inputs = append(inputs, InputValue{Name: "Beta", Status: InputAssumed, Value: market.Beta, HasValue: true, Source: "default", Note: "benchmark history unavailable"})
			notes = append(notes, "beta fallback used because benchmark history fetch failed: "+benchErr.Error())
		} else if beta, observations, ok := deriveBetaFromHistories(stockHistory, benchmarkHistory, options.BetaLookbackDays); ok {
			market.Beta = beta
			inputs = append(inputs, InputValue{Name: "Beta", Status: InputDerived, Value: beta, HasValue: true, Source: "stooq stock/benchmark daily history", Note: fmt.Sprintf("derived from %d matched daily returns", observations)})
		} else {
			market.Beta = 1.0
			inputs = append(inputs, InputValue{Name: "Beta", Status: InputAssumed, Value: market.Beta, HasValue: true, Source: "default", Note: "not enough matched returns"})
			notes = append(notes, "beta fallback used because there were not enough matched daily returns")
		}
	}

	if options.CostOfDebt != nil {
		market.CostOfDebt = *options.CostOfDebt
		inputs = append(inputs, InputValue{Name: "CostOfDebt", Status: InputAssumed, Value: market.CostOfDebt, HasValue: true, Source: "env override"})
	} else if costOfDebt, ok := deriveCostOfDebt(latest, financials); ok {
		if costOfDebt < market.RiskFreeRate {
			costOfDebt = market.RiskFreeRate
		}
		market.CostOfDebt = costOfDebt
		inputs = append(inputs, InputValue{Name: "CostOfDebt", Status: InputDerived, Value: costOfDebt, HasValue: true, Source: "sec.companyfacts interest expense / average debt"})
	} else {
		market.CostOfDebt = market.RiskFreeRate + defaultCostOfDebtSpread
		inputs = append(inputs, InputValue{Name: "CostOfDebt", Status: InputAssumed, Value: market.CostOfDebt, HasValue: true, Source: "risk free + spread", Note: fmt.Sprintf("fallback spread %.2f%%", defaultCostOfDebtSpread*100)})
	}

	marketCap := quote.Price * fin.SharesOut
	totalDebtForWeights := latest.TotalDebt
	if totalDebtForWeights < 0 {
		totalDebtForWeights = 0
	}
	totalCapital := marketCap + totalDebtForWeights
	if totalCapital <= 0 {
		return nil, fmt.Errorf("capital structure could not be derived for %s", symbol)
	}
	market.EquityWeight = marketCap / totalCapital
	market.DebtWeight = totalDebtForWeights / totalCapital
	inputs = append(inputs,
		InputValue{Name: "EquityWeight", Status: InputDerived, Value: market.EquityWeight, HasValue: true, Source: "market cap / (market cap + debt)"},
		InputValue{Name: "DebtWeight", Status: InputDerived, Value: market.DebtWeight, HasValue: true, Source: "debt / (market cap + debt)"},
	)

	assumptions := dcf.Assumptions{
		ForecastYears:    options.ForecastYears,
		TerminalGrowth:   options.TerminalGrowth,
		TargetPriceScale: options.PriceScale,
		TargetPriceUnit:  options.PriceUnit,
	}
	projection := deriveProjectionModel(financials)
	valuation, err := dcf.Value(fin, market, assumptions, projection)
	if err != nil {
		return nil, err
	}

	return &Result{
		Symbol:          symbol,
		CompanyName:     companyName,
		CIK:             cik,
		BenchmarkSymbol: options.BenchmarkSymbol,
		Quote:           quote,
		Inputs:          inputs,
		Notes:           notes,
		Financials:      financials,
		KeyMetrics:      buildKeyMetrics(quote, latest, marketCap, fin.NetDebt, projection.RevenueGrowth),
		Financial:       fin,
		Market:          market,
		Assumptions:     assumptions,
		Projection:      projection,
		Valuation:       valuation,
	}, nil
}

func resolveOptions(options AnalysisOptions) AnalysisOptions {
	if strings.TrimSpace(options.BenchmarkSymbol) == "" {
		options.BenchmarkSymbol = defaultBenchmarkSymbol
	}
	if options.ForecastYears <= 0 {
		options.ForecastYears = defaultForecastYears
	}
	if options.TerminalGrowth == 0 {
		options.TerminalGrowth = defaultTerminalGrowth
	}
	if options.PriceScale == 0 {
		options.PriceScale = defaultPriceScale
	}
	if strings.TrimSpace(options.PriceUnit) == "" {
		options.PriceUnit = defaultPriceUnit
	}
	if options.BetaLookbackDays <= 0 {
		options.BetaLookbackDays = defaultBetaLookbackDays
	}
	return options
}
