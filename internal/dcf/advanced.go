package dcf

import (
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync"
)

type ReverseDCFConfig struct {
	LowerGrowth   float64 `json:"lower_growth"`
	UpperGrowth   float64 `json:"upper_growth"`
	Epsilon       float64 `json:"epsilon"`
	MaxIterations int     `json:"max_iterations"`
}

type ReverseDCFResult struct {
	TargetPrice          float64          `json:"target_price"`
	ImpliedRevenueGrowth float64          `json:"implied_revenue_growth"`
	Iterations           int              `json:"iterations"`
	PriceError           float64          `json:"price_error"`
	Valuation            *ValuationResult `json:"valuation"`
}

type MonteCarloConfig struct {
	Iterations           int     `json:"iterations"`
	Workers              int     `json:"workers"`
	RevenueGrowthStdDev  float64 `json:"revenue_growth_std_dev"`
	WACCStdDev           float64 `json:"wacc_std_dev"`
	TerminalGrowthStdDev float64 `json:"terminal_growth_std_dev"`
	Seed1                uint64  `json:"seed1"`
	Seed2                uint64  `json:"seed2"`
}

type MonteCarloResult struct {
	RequestedIterations int     `json:"requested_iterations"`
	ValidIterations     int     `json:"valid_iterations"`
	InvalidIterations   int     `json:"invalid_iterations"`
	Mean                float64 `json:"mean"`
	Min                 float64 `json:"min"`
	Max                 float64 `json:"max"`
	P10                 float64 `json:"p10"`
	P50                 float64 `json:"p50"`
	P90                 float64 `json:"p90"`
}

func ReverseDCF(fin FinancialData, market MarketData, assumptions Assumptions, projection ProjectionModel, targetPrice float64, config ReverseDCFConfig) (*ReverseDCFResult, error) {
	config = normalizeReverseDCFConfig(config)
	if targetPrice <= 0 {
		return nil, fmt.Errorf("target price must be positive")
	}

	effectiveLowerGrowth := effectiveRevenueGrowth(projection, config.LowerGrowth)
	effectiveUpperGrowth := effectiveRevenueGrowth(projection, config.UpperGrowth)

	priceAt := func(growth float64) (*ValuationResult, error) {
		p := projection
		p.RevenueGrowth = growth
		return Value(fin, market, assumptions, p)
	}

	lowerValuation, err := priceAt(config.LowerGrowth)
	if err != nil {
		return nil, fmt.Errorf("lower growth valuation failed: %w", err)
	}
	upperValuation, err := priceAt(config.UpperGrowth)
	if err != nil {
		return nil, fmt.Errorf("upper growth valuation failed: %w", err)
	}

	lowerPrice := lowerValuation.TargetPrice
	upperPrice := upperValuation.TargetPrice
	increasing := upperPrice >= lowerPrice
	if !isBracketed(targetPrice, lowerPrice, upperPrice) {
		return nil, fmt.Errorf("target price %.4f is not bracketed by effective growth range [%.4f, %.4f] (requested [%.4f, %.4f]) -> prices [%.4f, %.4f]", targetPrice, effectiveLowerGrowth, effectiveUpperGrowth, config.LowerGrowth, config.UpperGrowth, lowerPrice, upperPrice)
	}

	left := config.LowerGrowth
	right := config.UpperGrowth
	bestGrowth := left
	bestValuation := lowerValuation
	bestError := math.Abs(lowerPrice - targetPrice)

	for iteration := 1; iteration <= config.MaxIterations; iteration++ {
		mid := (left + right) / 2
		valuation, err := priceAt(mid)
		if err != nil {
			return nil, fmt.Errorf("reverse dcf midpoint valuation failed: %w", err)
		}
		errorValue := valuation.TargetPrice - targetPrice
		absError := math.Abs(errorValue)
		if absError < bestError {
			bestError = absError
			bestGrowth = mid
			bestValuation = valuation
		}
		if absError <= config.Epsilon {
			return &ReverseDCFResult{
				TargetPrice:          targetPrice,
				ImpliedRevenueGrowth: mid,
				Iterations:           iteration,
				PriceError:           errorValue,
				Valuation:            valuation,
			}, nil
		}

		if increasing {
			if valuation.TargetPrice < targetPrice {
				left = mid
			} else {
				right = mid
			}
		} else {
			if valuation.TargetPrice > targetPrice {
				left = mid
			} else {
				right = mid
			}
		}
	}

	return &ReverseDCFResult{
		TargetPrice:          targetPrice,
		ImpliedRevenueGrowth: bestGrowth,
		Iterations:           config.MaxIterations,
		PriceError:           bestValuation.TargetPrice - targetPrice,
		Valuation:            bestValuation,
	}, nil
}

func MonteCarlo(fin FinancialData, market MarketData, assumptions Assumptions, projection ProjectionModel, config MonteCarloConfig) (*MonteCarloResult, error) {
	config = normalizeMonteCarloConfig(config)
	baseWACC := WACC(fin, market)
	baseCostOfEquity := CostOfEquity(market)

	type sample struct {
		price float64
		ok    bool
	}

	jobs := make(chan int)
	results := make(chan sample, config.Iterations)
	var wg sync.WaitGroup

	for worker := 0; worker < config.Workers; worker++ {
		wg.Add(1)
		go func(workerIndex int) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(config.Seed1+uint64(workerIndex)+1, config.Seed2+uint64(workerIndex)+11))
			for range jobs {
				sampledProjection := projection
				sampledProjection.RevenueGrowth += rng.NormFloat64() * config.RevenueGrowthStdDev
				sampledProjection = normalizeProjectionModel(sampledProjection)
				sampledAssumptions := assumptions
				sampledAssumptions.TerminalGrowth += rng.NormFloat64() * config.TerminalGrowthStdDev
				sampledWACC := baseWACC + (rng.NormFloat64() * config.WACCStdDev)
				forecast, err := buildForecast(fin, sampledAssumptions, sampledProjection)
				if err != nil || sampledWACC <= sampledAssumptions.TerminalGrowth || sampledWACC <= 0 {
					results <- sample{ok: false}
					continue
				}
				valuation, err := valueForecast(fin, sampledAssumptions, sampledProjection, forecast, sampledWACC, baseCostOfEquity)
				if err != nil || math.IsNaN(valuation.TargetPrice) || math.IsInf(valuation.TargetPrice, 0) {
					results <- sample{ok: false}
					continue
				}
				results <- sample{price: valuation.TargetPrice, ok: true}
			}
		}(worker)
	}

	for i := 0; i < config.Iterations; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)

	prices := make([]float64, 0, config.Iterations)
	invalidCount := 0
	for result := range results {
		if !result.ok {
			invalidCount++
			continue
		}
		prices = append(prices, result.price)
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("no valid monte carlo samples")
	}

	sort.Float64s(prices)
	mean := averageFloat64(prices)
	return &MonteCarloResult{
		RequestedIterations: config.Iterations,
		ValidIterations:     len(prices),
		InvalidIterations:   invalidCount,
		Mean:                mean,
		Min:                 prices[0],
		Max:                 prices[len(prices)-1],
		P10:                 percentile(prices, 0.10),
		P50:                 percentile(prices, 0.50),
		P90:                 percentile(prices, 0.90),
	}, nil
}

func buildForecast(fin FinancialData, assumptions Assumptions, model ProjectionModel) ([]ForecastYear, error) {
	if assumptions.ForecastYears <= 0 {
		return nil, fmt.Errorf("forecast years must be positive")
	}
	if fin.Revenue <= 0 {
		return nil, fmt.Errorf("revenue must be positive")
	}

	forecast := make([]ForecastYear, 0, assumptions.ForecastYears)
	revenue := fin.Revenue
	for year := 1; year <= assumptions.ForecastYears; year++ {
		revenue *= (1 + model.RevenueGrowth)
		ebit := revenue * model.EBITMargin
		dna := revenue * model.DNAMargin
		capex := revenue * model.CapExMargin
		deltaNWC := revenue * model.NWCMargin
		fcf := (ebit * (1 - fin.EffectiveTax)) + dna - capex - deltaNWC
		forecast = append(forecast, ForecastYear{
			Year:        year,
			Revenue:     revenue,
			EBIT:        ebit,
			DnA:         dna,
			CapEx:       capex,
			ChangeInNWC: deltaNWC,
			FCF:         fcf,
		})
	}
	return forecast, nil
}

func valueForecast(fin FinancialData, assumptions Assumptions, model ProjectionModel, forecast []ForecastYear, wacc float64, costOfEquity float64) (*ValuationResult, error) {
	if fin.SharesOut <= 0 {
		return nil, fmt.Errorf("shares out must be positive")
	}
	assumptions = normalizeAssumptions(assumptions)
	if len(forecast) == 0 {
		return nil, fmt.Errorf("forecast is empty")
	}
	if wacc <= assumptions.TerminalGrowth {
		return nil, fmt.Errorf("wacc %.6f must be greater than terminal growth %.6f", wacc, assumptions.TerminalGrowth)
	}
	if wacc <= 0 {
		return nil, fmt.Errorf("wacc must be positive")
	}

	pricedForecast := make([]ForecastYear, len(forecast))
	copy(pricedForecast, forecast)
	var pvSum float64
	for index := range pricedForecast {
		discountFactor := math.Pow(1+wacc, float64(pricedForecast[index].Year))
		pricedForecast[index].DiscountFactor = discountFactor
		pricedForecast[index].PresentValue = pricedForecast[index].FCF / discountFactor
		pvSum += pricedForecast[index].PresentValue
	}
	lastFCF := pricedForecast[len(pricedForecast)-1].FCF
	terminalValue := (lastFCF * (1 + assumptions.TerminalGrowth)) / (wacc - assumptions.TerminalGrowth)
	terminalPresentValue := terminalValue / math.Pow(1+wacc, float64(assumptions.ForecastYears))
	enterpriseValue := pvSum + terminalPresentValue
	equityValue := enterpriseValue - fin.NetDebt
	targetPriceRaw := equityValue / fin.SharesOut
	targetPrice := targetPriceRaw * assumptions.TargetPriceScale
	return &ValuationResult{
		BaseFCF:              FCF(fin),
		CostOfEquity:         costOfEquity,
		WACC:                 wacc,
		TerminalValue:        terminalValue,
		TerminalPresentValue: terminalPresentValue,
		EnterpriseValue:      enterpriseValue,
		EquityValue:          equityValue,
		TargetPriceRaw:       targetPriceRaw,
		TargetPriceScale:     assumptions.TargetPriceScale,
		TargetPriceUnit:      assumptions.TargetPriceUnit,
		TargetPrice:          targetPrice,
		Forecast:             pricedForecast,
		Projection:           model,
		Assumptions:          assumptions,
	}, nil
}

func normalizeReverseDCFConfig(config ReverseDCFConfig) ReverseDCFConfig {
	if config.LowerGrowth == 0 && config.UpperGrowth == 0 {
		config.LowerGrowth = -0.50
		config.UpperGrowth = 1.00
	}
	if config.Epsilon <= 0 {
		config.Epsilon = 0.001
	}
	if config.MaxIterations <= 0 {
		config.MaxIterations = 100
	}
	return config
}

func effectiveRevenueGrowth(model ProjectionModel, growth float64) float64 {
	model.RevenueGrowth = growth
	return normalizeProjectionModel(model).RevenueGrowth
}

func normalizeMonteCarloConfig(config MonteCarloConfig) MonteCarloConfig {
	if config.Iterations <= 0 {
		config.Iterations = 10000
	}
	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
		if config.Workers < 1 {
			config.Workers = 1
		}
	}
	if config.RevenueGrowthStdDev <= 0 {
		config.RevenueGrowthStdDev = 0.02
	}
	if config.WACCStdDev <= 0 {
		config.WACCStdDev = 0.01
	}
	if config.TerminalGrowthStdDev <= 0 {
		config.TerminalGrowthStdDev = 0.005
	}
	if config.Seed1 == 0 {
		config.Seed1 = 42
	}
	if config.Seed2 == 0 {
		config.Seed2 = 1729
	}
	return config
}

func isBracketed(target float64, left float64, right float64) bool {
	low := math.Min(left, right)
	high := math.Max(left, right)
	return target >= low && target <= high
}

func averageFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func percentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	if p <= 0 {
		return sortedValues[0]
	}
	if p >= 1 {
		return sortedValues[len(sortedValues)-1]
	}
	position := p * float64(len(sortedValues)-1)
	lowerIndex := int(math.Floor(position))
	upperIndex := int(math.Ceil(position))
	if lowerIndex == upperIndex {
		return sortedValues[lowerIndex]
	}
	weight := position - float64(lowerIndex)
	return sortedValues[lowerIndex] + ((sortedValues[upperIndex] - sortedValues[lowerIndex]) * weight)
}
