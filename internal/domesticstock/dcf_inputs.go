package domesticstock

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/dcf"
	"github.com/fifi/internal/domesticbond"
	"github.com/fifi/internal/marketpremium"
)

func buildDCFInput(name string, row map[string]any, key string, status DCFInputStatus, source string, note string) DCFInputValue {
	value, ok := parseFloat(row[key])
	if !ok {
		return DCFInputValue{
			Name:   name,
			Status: DCFInputMissing,
			Source: source,
			Note:   note,
		}
	}

	return DCFInputValue{
		Name:     name,
		Status:   status,
		Value:    value,
		HasValue: true,
		Source:   source,
		Note:     note,
	}
}

func sortedFinanceRows(resp *auth.RESTResponse) []map[string]any {
	rows := toRows(resp.Body["output"])
	sort.SliceStable(rows, func(i, j int) bool {
		return fieldStringMap(rows[i], "stac_yymm") > fieldStringMap(rows[j], "stac_yymm")
	})
	return rows
}

func fieldStringMap(row map[string]any, key string) string {
	if row == nil {
		return ""
	}
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(toString(value))
}

func deriveEffectiveTax(row map[string]any) (float64, bool) {
	preTaxIncome, ok := parseFloat(row["op_prfi"])
	if !ok || preTaxIncome <= 0 {
		return 0, false
	}

	netIncome, ok := parseFloat(row["thtr_ntin"])
	if !ok {
		return 0, false
	}

	tax := 1 - (netIncome / preTaxIncome)
	if tax < 0 {
		tax = 0
	}
	if tax > 1 {
		tax = 1
	}
	return tax, true
}

func deriveCapEx(balanceRows []map[string]any, incomeRow map[string]any) (float64, bool) {
	if len(balanceRows) < 2 {
		return 0, false
	}

	latestFixedAssets, ok := parseFloat(balanceRows[0]["fxas"])
	if !ok {
		return 0, false
	}
	prevFixedAssets, ok := parseFloat(balanceRows[1]["fxas"])
	if !ok {
		return 0, false
	}
	depreciation, ok := parseFloat(incomeRow["depr_cost"])
	if !ok {
		return 0, false
	}

	capex := (latestFixedAssets - prevFixedAssets) + depreciation
	if capex < 0 {
		capex = 0
	}
	return capex, true
}

func deriveChangeInNWC(balanceRows []map[string]any) (float64, bool) {
	if len(balanceRows) < 2 {
		return 0, false
	}

	latestCurrentAssets, ok := parseFloat(balanceRows[0]["cras"])
	if !ok {
		return 0, false
	}
	latestCurrentLiabilities, ok := parseFloat(balanceRows[0]["flow_lblt"])
	if !ok {
		return 0, false
	}
	prevCurrentAssets, ok := parseFloat(balanceRows[1]["cras"])
	if !ok {
		return 0, false
	}
	prevCurrentLiabilities, ok := parseFloat(balanceRows[1]["flow_lblt"])
	if !ok {
		return 0, false
	}

	latestNWC := latestCurrentAssets - latestCurrentLiabilities
	prevNWC := prevCurrentAssets - prevCurrentLiabilities
	return latestNWC - prevNWC, true
}

func deriveCapitalWeights(row map[string]any) (float64, float64, bool) {
	totalAssets, ok := parseFloat(row["total_aset"])
	if !ok || totalAssets <= 0 {
		return 0, 0, false
	}
	totalCapital, ok := parseFloat(row["total_cptl"])
	if !ok {
		return 0, 0, false
	}
	totalLiabilities, ok := parseFloat(row["total_lblt"])
	if !ok {
		return 0, 0, false
	}

	return totalCapital / totalAssets, totalLiabilities / totalAssets, true
}

func missingInputNames(inputs []DCFInputValue, names ...string) []string {
	lookup := make(map[string]DCFInputValue, len(inputs))
	for _, input := range inputs {
		lookup[input.Name] = input
	}

	missing := make([]string, 0, len(names))
	for _, name := range names {
		input, ok := lookup[name]
		if !ok || input.Status == DCFInputMissing {
			missing = append(missing, name)
		}
	}
	return missing
}

func (s *Service) buildDCFInputBundle(ctx context.Context, symbol string, options DCFValuationOptions) (*dcfInputBundle, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	options = resolveDCFValuationOptions(options)
	bundle := &dcfInputBundle{
		Symbol:   symbol,
		Division: options.DivClsCode,
		Notes:    make([]string, 0, 8),
		Inputs:   make([]DCFInputValue, 0, 16),
		Assumptions: dcf.Assumptions{
			ForecastYears:  options.ForecastYears,
			TerminalGrowth: options.TerminalGrowth,
		},
	}

	priceResp, err := s.InquirePrice(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("inquire-price failed: %w", err)
	}
	bundle.PriceRow = firstOutputRow(priceResp, "output")
	if bundle.PriceRow == nil {
		return nil, fmt.Errorf("inquire-price output missing")
	}

	balanceResp, err := s.FinanceBalanceSheet(ctx, symbol, options.DivClsCode)
	if err != nil {
		return nil, fmt.Errorf("finance balance-sheet failed: %w", err)
	}
	bundle.BalanceRows = sortedFinanceRows(balanceResp)
	if len(bundle.BalanceRows) == 0 {
		return nil, fmt.Errorf("balance-sheet output missing")
	}

	incomeResp, err := s.FinanceIncomeStatement(ctx, symbol, options.DivClsCode)
	if err != nil {
		return nil, fmt.Errorf("finance income-statement failed: %w", err)
	}
	bundle.IncomeRows = sortedFinanceRows(incomeResp)
	if len(bundle.IncomeRows) == 0 {
		return nil, fmt.Errorf("income-statement output missing")
	}

	var otherRows []map[string]any
	if resp, reqErr := s.FinanceOtherMajorRatios(ctx, symbol, options.DivClsCode); reqErr == nil {
		otherRows = sortedFinanceRows(resp)
	} else {
		bundle.Notes = append(bundle.Notes, "other-major-ratios unavailable: "+reqErr.Error())
	}

	var stabilityRows []map[string]any
	if resp, reqErr := s.FinanceStabilityRatio(ctx, symbol, options.DivClsCode); reqErr == nil {
		stabilityRows = sortedFinanceRows(resp)
	} else {
		bundle.Notes = append(bundle.Notes, "stability-ratio unavailable: "+reqErr.Error())
	}

	latestBalance := bundle.BalanceRows[0]
	latestIncome := bundle.IncomeRows[0]
	latestOther := firstRowOrNil(otherRows)
	latestStability := firstRowOrNil(stabilityRows)

	bundle.Inputs = append(bundle.Inputs,
		buildDCFInput("Revenue", latestIncome, "sale_account", DCFInputExact, "income-statement.output.sale_account", ""),
		buildDCFInput("EBIT", latestIncome, "bsop_prti", DCFInputExact, "income-statement.output.bsop_prti", ""),
		buildDCFInput("DnA", latestIncome, "depr_cost", DCFInputExact, "income-statement.output.depr_cost", ""),
		buildDCFInput("SharesOut", bundle.PriceRow, "lstn_stcn", DCFInputExact, "inquire-price.output.lstn_stcn", ""),
	)

	if marketCap, ok := parseFloat(bundle.PriceRow["hts_avls"]); ok && marketCap > 0 {
		bundle.MarketCap = marketCap
		bundle.HasMarketCap = true
		bundle.Notes = append(bundle.Notes, "market cap sourced from inquire-price.output.hts_avls")
	}
	if currentPrice, ok := firstFloat(bundle.PriceRow, "stck_prpr", "stck_clpr"); ok && currentPrice > 0 {
		bundle.CurrentPrice = currentPrice
	}

	if tax, ok := deriveEffectiveTax(latestIncome); ok {
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{
			Name:     "EffectiveTax",
			Status:   DCFInputDerived,
			Value:    tax,
			HasValue: true,
			Source:   "income-statement.output.(op_prfi,thtr_ntin)",
			Note:     "derived as 1 - thtr_ntin/op_prfi",
		})
		bundle.Financial.EffectiveTax = tax
	} else {
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "EffectiveTax", Status: DCFInputMissing, Source: "income-statement.output.(op_prfi,thtr_ntin)", Note: "pre-tax income or net income missing"})
	}

	if revenueInput := findDCFInput(bundle.Inputs, "Revenue"); revenueInput.HasValue {
		bundle.Financial.Revenue = revenueInput.Value
	}
	if ebitInput := findDCFInput(bundle.Inputs, "EBIT"); ebitInput.HasValue {
		bundle.Financial.EBIT = ebitInput.Value
	}
	if dnaInput := findDCFInput(bundle.Inputs, "DnA"); dnaInput.HasValue {
		bundle.Financial.DnA = dnaInput.Value
	}
	if sharesInput := findDCFInput(bundle.Inputs, "SharesOut"); sharesInput.HasValue {
		bundle.Financial.SharesOut = sharesInput.Value
	}

	if capex, ok := deriveCapEx(bundle.BalanceRows, latestIncome); ok {
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "CapEx", Status: DCFInputDerived, Value: capex, HasValue: true, Source: "balance-sheet.output.fxas + income-statement.output.depr_cost", Note: "proxy: delta fixed assets + depreciation"})
		bundle.Financial.CapEx = capex
	} else {
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "CapEx", Status: DCFInputMissing, Source: "balance-sheet.output.fxas", Note: "requires at least two balance-sheet periods and depreciation"})
	}

	if nwc, ok := deriveChangeInNWC(bundle.BalanceRows); ok {
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "ChangeInNWC", Status: DCFInputDerived, Value: nwc, HasValue: true, Source: "balance-sheet.output.(cras,flow_lblt)", Note: "derived from delta(current assets - current liabilities)"})
		bundle.Financial.ChangeInNWC = nwc
	} else {
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "ChangeInNWC", Status: DCFInputMissing, Source: "balance-sheet.output.(cras,flow_lblt)", Note: "requires at least two balance-sheet periods"})
	}

	if input, ok := resolveNetDebtInput(options, bundle.PriceRow, latestOther, latestStability, latestBalance); ok {
		bundle.Inputs = append(bundle.Inputs, input)
		bundle.Financial.NetDebt = input.Value
	} else {
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "NetDebt", Status: DCFInputMissing, Source: "other-major-ratios + inquire-price + stability-ratio", Note: "could not derive a usable net debt proxy"})
	}

	if eqWeight, debtWeight, eqInput, debtInput, ok := resolveCapitalWeights(bundle.MarketCap, bundle.HasMarketCap, findDCFInput(bundle.Inputs, "NetDebt"), latestBalance); ok {
		bundle.Inputs = append(bundle.Inputs, eqInput, debtInput)
		bundle.Market.EquityWeight = eqWeight
		bundle.Market.DebtWeight = debtWeight
	} else {
		bundle.Inputs = append(bundle.Inputs,
			DCFInputValue{Name: "EquityWeight", Status: DCFInputMissing, Source: "market cap + net debt or balance-sheet.output.(total_cptl,total_aset)"},
			DCFInputValue{Name: "DebtWeight", Status: DCFInputMissing, Source: "market cap + net debt or balance-sheet.output.(total_cptl,total_aset)"},
		)
	}

	if input, ok := resolveRiskFreeRateInput(options); ok {
		bundle.Inputs = append(bundle.Inputs, input)
		bundle.Market.RiskFreeRate = input.Value
	} else {
		bondInput, bondNote, bondOK := s.resolveRiskFreeRateFromBond(ctx, options)
		if bondOK {
			bundle.Inputs = append(bundle.Inputs, bondInput)
			bundle.Market.RiskFreeRate = bondInput.Value
			if bondNote != "" {
				bundle.Notes = append(bundle.Notes, bondNote)
			}
		} else {
			if bondNote != "" {
				bundle.Notes = append(bundle.Notes, bondNote)
			}
			if resp, reqErr := s.CompInterest(ctx); reqErr == nil {
				if input, found := deriveRiskFreeRateFromCompInterest(resp); found {
					bundle.Inputs = append(bundle.Inputs, input)
					bundle.Market.RiskFreeRate = input.Value
				} else {
					bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "RiskFreeRate", Status: DCFInputMissing, Source: "comp-interest.output1/output2", Note: "10Y treasury row not found"})
				}
			} else {
				bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "RiskFreeRate", Status: DCFInputMissing, Source: "comp-interest", Note: reqErr.Error()})
			}
		}
	}

	if input, ok := resolveMarketPremiumInput(options); ok {
		bundle.Inputs = append(bundle.Inputs, input)
		bundle.Market.MarketPremium = input.Value
	} else {
		providerInput, providerNote, providerOK := s.resolveMarketPremiumFromProvider(ctx)
		if providerOK {
			bundle.Inputs = append(bundle.Inputs, providerInput)
			bundle.Market.MarketPremium = providerInput.Value
			if providerNote != "" {
				bundle.Notes = append(bundle.Notes, providerNote)
			}
		} else {
			fallback := defaultDCFMarketPremium
			bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "MarketPremium", Status: DCFInputAssumed, Value: fallback, HasValue: true, Source: dcfMarketPremiumEnvKey, Note: "default assumption 5.50%"})
			bundle.Market.MarketPremium = fallback
			if providerNote != "" {
				bundle.Notes = append(bundle.Notes, providerNote)
			}
		}
	}

	if input, ok := resolveBetaInput(options); ok {
		bundle.Inputs = append(bundle.Inputs, input)
		bundle.Market.Beta = input.Value
	} else {
		lookbackFromDate := time.Now().AddDate(0, 0, -options.BetaLookbackDays).Format("20060102")
		stockResp, stockErr := s.InquireDailyItemChartPrice(ctx, symbol, lookbackFromDate, time.Now().Format("20060102"), "D", "1")
		if stockErr != nil {
			bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "Beta", Status: DCFInputMissing, Source: "inquire-daily-itemchartprice", Note: stockErr.Error()})
		} else if indexRows, indexErr := s.InquireIndexDailyPrice(ctx, options.IndexCode, lookbackFromDate); indexErr != nil {
			bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "Beta", Status: DCFInputMissing, Source: "inquire-index-daily-price", Note: indexErr.Error()})
		} else if beta, observations, ok := deriveBetaFromRows(toRows(stockResp.Body["output2"]), indexRows); ok {
			bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "Beta", Status: DCFInputDerived, Value: beta, HasValue: true, Source: "inquire-daily-itemchartprice + inquire-index-daily-price", Note: fmt.Sprintf("derived from %d matched daily returns", observations)})
			bundle.Market.Beta = beta
		} else {
			bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "Beta", Status: DCFInputMissing, Source: "inquire-daily-itemchartprice + inquire-index-daily-price", Note: "not enough matched returns for beta"})
		}
	}

	if input, ok := resolveCostOfDebtInput(options); ok {
		bundle.Inputs = append(bundle.Inputs, input)
		bundle.Market.CostOfDebt = input.Value
	} else if input, ok := deriveCostOfDebtInput(latestIncome, latestStability, latestBalance, bundle.Market.RiskFreeRate); ok {
		bundle.Inputs = append(bundle.Inputs, input)
		bundle.Market.CostOfDebt = input.Value
	} else if bundle.Market.RiskFreeRate > 0 {
		spread := envFloatOrDefault(dcfCreditSpreadEnvKey, defaultDCFCreditSpread)
		costOfDebt := bundle.Market.RiskFreeRate + spread
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "CostOfDebt", Status: DCFInputAssumed, Value: costOfDebt, HasValue: true, Source: dcfCreditSpreadEnvKey, Note: fmt.Sprintf("assumed as risk free rate + %.2f%% credit spread", spread*100)})
		bundle.Market.CostOfDebt = costOfDebt
	} else {
		bundle.Inputs = append(bundle.Inputs, DCFInputValue{Name: "CostOfDebt", Status: DCFInputMissing, Source: "stability-ratio + income-statement", Note: "requires risk free rate or debt proxy"})
	}

	bundle.Projection = deriveProjectionModel(bundle.IncomeRows, bundle.BalanceRows)
	return bundle, nil
}

func resolveDCFValuationOptions(options DCFValuationOptions) DCFValuationOptions {
	if strings.TrimSpace(options.DivClsCode) == "" {
		options.DivClsCode = defaultFinanceDivClsCode
	}
	if options.ForecastYears <= 0 {
		options.ForecastYears = int(envFloatOrDefault(dcfForecastYearsEnvKey, defaultDCFForecastYears))
	}
	if options.TerminalGrowth == 0 {
		options.TerminalGrowth = envFloatOrDefault(dcfTerminalGrowthEnvKey, defaultDCFTerminalGrowth)
	}
	if strings.TrimSpace(options.IndexCode) == "" {
		options.IndexCode = strings.TrimSpace(os.Getenv(dcfIndexCodeEnvKey))
		if options.IndexCode == "" {
			options.IndexCode = defaultDCFIndexCode
		}
	}
	if options.BetaLookbackDays <= 0 {
		options.BetaLookbackDays = int(envFloatOrDefault(dcfBetaLookbackEnvKey, defaultDCFBetaLookback))
	}
	if options.RiskFreeRate == nil {
		options.RiskFreeRate = envOptionalFloat(dcfRiskFreeRateEnvKey)
	}
	if strings.TrimSpace(options.RiskFreeBondCode) == "" {
		options.RiskFreeBondCode = strings.TrimSpace(os.Getenv(dcfRiskFreeBondCodeEnvKey))
	}
	if strings.TrimSpace(options.RiskFreeBondDiv) == "" {
		options.RiskFreeBondDiv = strings.TrimSpace(os.Getenv(dcfRiskFreeBondDivEnvKey))
		if options.RiskFreeBondDiv == "" {
			options.RiskFreeBondDiv = defaultDCFRiskFreeBondDiv
		}
	}
	if options.Beta == nil {
		options.Beta = envOptionalFloat(dcfBetaEnvKey)
	}
	if options.MarketPremium == nil {
		options.MarketPremium = envOptionalFloat(dcfMarketPremiumEnvKey)
	}
	if options.CostOfDebt == nil {
		options.CostOfDebt = envOptionalFloat(dcfCostOfDebtEnvKey)
	}
	if options.NetDebt == nil {
		options.NetDebt = envOptionalFloat(dcfNetDebtEnvKey)
	}
	return options
}

func resolveRiskFreeRateInput(options DCFValuationOptions) (DCFInputValue, bool) {
	if options.RiskFreeRate == nil {
		return DCFInputValue{}, false
	}
	return DCFInputValue{Name: "RiskFreeRate", Status: DCFInputAssumed, Value: *options.RiskFreeRate, HasValue: true, Source: dcfRiskFreeRateEnvKey, Note: "env override"}, true
}

func (s *Service) resolveRiskFreeRateFromBond(ctx context.Context, options DCFValuationOptions) (DCFInputValue, string, bool) {
	bondCode := strings.TrimSpace(options.RiskFreeBondCode)
	if bondCode == "" {
		return DCFInputValue{}, "", false
	}

	resp, err := domesticbond.NewService(s.client).InquirePrice(ctx, options.RiskFreeBondDiv, bondCode)
	if err != nil {
		return DCFInputValue{}, "domestic-bond inquire-price unavailable: " + err.Error(), false
	}

	input, ok := deriveRiskFreeRateFromBondPrice(resp, bondCode)
	if !ok {
		return DCFInputValue{}, "domestic-bond inquire-price missing ernn_rate for " + bondCode, false
	}
	return input, "", true
}

func resolveBetaInput(options DCFValuationOptions) (DCFInputValue, bool) {
	if options.Beta == nil {
		return DCFInputValue{}, false
	}
	return DCFInputValue{Name: "Beta", Status: DCFInputAssumed, Value: *options.Beta, HasValue: true, Source: dcfBetaEnvKey, Note: "env override"}, true
}

func resolveMarketPremiumInput(options DCFValuationOptions) (DCFInputValue, bool) {
	if options.MarketPremium == nil {
		return DCFInputValue{}, false
	}
	return DCFInputValue{Name: "MarketPremium", Status: DCFInputAssumed, Value: *options.MarketPremium, HasValue: true, Source: dcfMarketPremiumEnvKey, Note: "env override"}, true
}

func (s *Service) resolveMarketPremiumFromProvider(ctx context.Context) (DCFInputValue, string, bool) {
	resolver := marketpremium.Resolver{HTTPClient: s.client.Client}
	value, err := resolver.Resolve(ctx)
	if err != nil {
		return DCFInputValue{}, "market premium provider unavailable: " + err.Error(), false
	}
	if value == nil || value.Rate <= 0 {
		return DCFInputValue{}, "market premium provider returned empty rate", false
	}

	note := strings.TrimSpace(value.Note)
	if value.AsOf != "" {
		if note == "" {
			note = "as of " + value.AsOf
		} else {
			note = note + " | as of " + value.AsOf
		}
	}

	input := DCFInputValue{
		Name:     "MarketPremium",
		Status:   DCFInputExact,
		Value:    value.Rate,
		HasValue: true,
		Source:   "external-market-premium." + value.Provider,
		Note:     note,
	}
	sourceNote := strings.TrimSpace(value.Source)
	if value.URL != "" {
		if sourceNote == "" {
			sourceNote = value.URL
		} else {
			sourceNote = sourceNote + " | " + value.URL
		}
	}
	return input, sourceNote, true
}

func resolveCostOfDebtInput(options DCFValuationOptions) (DCFInputValue, bool) {
	if options.CostOfDebt == nil {
		return DCFInputValue{}, false
	}
	return DCFInputValue{Name: "CostOfDebt", Status: DCFInputAssumed, Value: *options.CostOfDebt, HasValue: true, Source: dcfCostOfDebtEnvKey, Note: "env override"}, true
}

func resolveNetDebtInput(options DCFValuationOptions, priceRow map[string]any, otherRow map[string]any, stabilityRow map[string]any, balanceRow map[string]any) (DCFInputValue, bool) {
	if options.NetDebt != nil {
		return DCFInputValue{Name: "NetDebt", Status: DCFInputAssumed, Value: *options.NetDebt, HasValue: true, Source: dcfNetDebtEnvKey, Note: "env override"}, true
	}

	marketCap, marketCapOK := parseFloat(priceRow["hts_avls"])
	if marketCapOK && marketCap > 0 {
		ebitda, ebitdaOK := parseFloat(otherRow["ebitda"])
		evMultiple, multipleOK := parseFloat(otherRow["ev_ebitda"])
		if ebitdaOK && multipleOK && ebitda > 0 && evMultiple > 0 {
			ev := ebitda * evMultiple
			netDebt := ev - marketCap
			if ev > marketCap*0.25 && ev < marketCap*10 {
				return DCFInputValue{Name: "NetDebt", Status: DCFInputDerived, Value: netDebt, HasValue: true, Source: "other-major-ratios.output.(ebitda,ev_ebitda) + inquire-price.output.hts_avls", Note: "proxy via EV - market cap"}, true
			}
		}
	}

	if totalAssets, totalAssetsOK := parseFloat(balanceRow["total_aset"]); totalAssetsOK && totalAssets > 0 {
		if bramDepn, ok := parseFloat(stabilityRow["bram_depn"]); ok && bramDepn > 0 {
			debtProxy := totalAssets * percentToRatio(bramDepn)
			return DCFInputValue{Name: "NetDebt", Status: DCFInputDerived, Value: debtProxy, HasValue: true, Source: "stability-ratio.output.bram_depn + balance-sheet.output.total_aset", Note: "gross debt proxy; cash equivalents not isolated"}, true
		}
	}

	return DCFInputValue{}, false
}

func resolveCapitalWeights(marketCap float64, hasMarketCap bool, netDebtInput DCFInputValue, balanceRow map[string]any) (float64, float64, DCFInputValue, DCFInputValue, bool) {
	if hasMarketCap && netDebtInput.HasValue {
		debtComponent := netDebtInput.Value
		if debtComponent < 0 {
			debtComponent = 0
		}
		totalCapital := marketCap + debtComponent
		if totalCapital > 0 {
			equityWeight := marketCap / totalCapital
			debtWeight := debtComponent / totalCapital
			return equityWeight, debtWeight,
				DCFInputValue{Name: "EquityWeight", Status: DCFInputDerived, Value: equityWeight, HasValue: true, Source: "market cap / (market cap + max(net debt,0))", Note: "market-value capital structure proxy"},
				DCFInputValue{Name: "DebtWeight", Status: DCFInputDerived, Value: debtWeight, HasValue: true, Source: "max(net debt,0) / (market cap + max(net debt,0))", Note: "market-value capital structure proxy"},
				true
		}
	}

	if equityWeight, debtWeight, ok := deriveCapitalWeights(balanceRow); ok {
		return equityWeight, debtWeight,
			DCFInputValue{Name: "EquityWeight", Status: DCFInputDerived, Value: equityWeight, HasValue: true, Source: "balance-sheet.output.(total_cptl,total_aset)", Note: "proxy using book capital structure"},
			DCFInputValue{Name: "DebtWeight", Status: DCFInputDerived, Value: debtWeight, HasValue: true, Source: "balance-sheet.output.(total_lblt,total_aset)", Note: "proxy using total liabilities, not interest-bearing debt only"},
			true
	}

	return 0, 0, DCFInputValue{}, DCFInputValue{}, false
}

func deriveRiskFreeRateFromCompInterest(resp *auth.RESTResponse) (DCFInputValue, bool) {
	rows := append(toRows(resp.Body["output1"]), toRows(resp.Body["output2"])...)
	if len(rows) == 0 {
		return DCFInputValue{}, false
	}

	nameHint := strings.TrimSpace(os.Getenv(dcfRiskFreeNameHintEnvKey))
	if nameHint == "" {
		nameHint = "국고채 10년"
	}

	for _, row := range rows {
		name := fieldStringMap(row, "hts_kor_isnm")
		if name == nameHint {
			if input, ok := riskFreeRateInputFromRow(row); ok {
				input.Note = name + " (name hint exact match)"
				return input, true
			}
		}
	}

	bestRow := map[string]any(nil)
	fallbackRow := map[string]any(nil)
	for _, row := range rows {
		name := fieldStringMap(row, "hts_kor_isnm")
		if name == "" {
			continue
		}
		if strings.Contains(name, "10년") {
			if fallbackRow == nil {
				fallbackRow = row
			}
			if strings.Contains(name, "국고채") || strings.Contains(name, "국채") {
				bestRow = row
				break
			}
		}
	}
	if bestRow == nil {
		bestRow = fallbackRow
	}
	if bestRow == nil {
		return DCFInputValue{}, false
	}

	return riskFreeRateInputFromRow(bestRow)
}

func deriveRiskFreeRateFromBondPrice(resp *auth.RESTResponse, bondCode string) (DCFInputValue, bool) {
	row := firstOutputRow(resp, "output")
	if row == nil {
		return DCFInputValue{}, false
	}

	rate, ok := parseFloat(row["ernn_rate"])
	if !ok || rate <= 0 {
		return DCFInputValue{}, false
	}
	rate = percentToRatio(rate)

	noteParts := make([]string, 0, 2)
	if name := fieldStringMap(row, "hts_kor_isnm"); name != "" {
		noteParts = append(noteParts, name)
	}
	if iscd := fieldStringMap(row, "stnd_iscd"); iscd != "" {
		noteParts = append(noteParts, iscd)
	}
	note := strings.Join(noteParts, " | ")
	if note == "" {
		note = bondCode
	}

	return DCFInputValue{
		Name:     "RiskFreeRate",
		Status:   DCFInputExact,
		Value:    rate,
		HasValue: true,
		Source:   "domestic-bond.inquire-price.output.(stnd_iscd,ernn_rate)",
		Note:     note,
	}, true
}

func riskFreeRateInputFromRow(row map[string]any) (DCFInputValue, bool) {
	rate, ok := parseFloat(row["bond_mnrt_prpr"])
	if !ok || rate <= 0 {
		return DCFInputValue{}, false
	}
	rate = percentToRatio(rate)
	return DCFInputValue{Name: "RiskFreeRate", Status: DCFInputExact, Value: rate, HasValue: true, Source: "comp-interest.output.(hts_kor_isnm,bond_mnrt_prpr)", Note: fieldStringMap(row, "hts_kor_isnm")}, true
}

func deriveCostOfDebtInput(incomeRow map[string]any, stabilityRow map[string]any, balanceRow map[string]any, riskFreeRate float64) (DCFInputValue, bool) {
	nonOperatingExpense, expOK := parseFloat(incomeRow["bsop_non_expn"])
	bramDepn, bramOK := parseFloat(stabilityRow["bram_depn"])
	totalAssets, assetsOK := parseFloat(balanceRow["total_aset"])
	if expOK && bramOK && assetsOK && totalAssets > 0 {
		debtProxy := totalAssets * percentToRatio(bramDepn)
		if debtProxy > 0 {
			costOfDebt := math.Abs(nonOperatingExpense) / debtProxy
			if costOfDebt > 0 {
				if riskFreeRate > 0 && costOfDebt < riskFreeRate {
					costOfDebt = riskFreeRate
				}
				if costOfDebt <= 0.30 {
					return DCFInputValue{Name: "CostOfDebt", Status: DCFInputDerived, Value: costOfDebt, HasValue: true, Source: "income-statement.output.bsop_non_expn + stability-ratio.output.bram_depn", Note: "proxy using non-operating expense over debt proxy"}, true
				}
			}
		}
	}
	return DCFInputValue{}, false
}

func deriveProjectionModel(incomeRows []map[string]any, balanceRows []map[string]any) dcf.ProjectionModel {
	model := dcf.ProjectionModel{
		RevenueGrowth: 0.05,
		EBITMargin:    0.10,
		DNAMargin:     0.03,
		CapExMargin:   0.04,
		NWCMargin:     0.01,
	}

	if growth, ok := deriveRevenueGrowth(incomeRows); ok {
		model.RevenueGrowth = growth
	}
	if margin, ok := averageRatio(incomeRows, "bsop_prti", "sale_account"); ok {
		model.EBITMargin = margin
	}
	if margin, ok := averageRatio(incomeRows, "depr_cost", "sale_account"); ok {
		model.DNAMargin = math.Max(margin, 0)
	}
	if margin, ok := averageCapExMargin(incomeRows, balanceRows); ok {
		model.CapExMargin = math.Max(margin, 0)
	}
	if margin, ok := averageNWCMargin(incomeRows, balanceRows); ok {
		model.NWCMargin = margin
	}

	return model
}

func deriveRevenueGrowth(incomeRows []map[string]any) (float64, bool) {
	valid := make([]float64, 0, len(incomeRows))
	for _, row := range incomeRows {
		revenue, ok := parseFloat(row["sale_account"])
		if !ok || revenue <= 0 {
			continue
		}
		valid = append(valid, revenue)
		if len(valid) >= 5 {
			break
		}
	}
	if len(valid) < 2 {
		return 0, false
	}
	latest := valid[0]
	oldest := valid[len(valid)-1]
	years := float64(len(valid) - 1)
	if oldest <= 0 || years <= 0 {
		return 0, false
	}
	growth := math.Pow(latest/oldest, 1/years) - 1
	if math.IsNaN(growth) || math.IsInf(growth, 0) {
		return 0, false
	}
	return growth, true
}

func averageRatio(rows []map[string]any, numeratorKey string, denominatorKey string) (float64, bool) {
	var sum float64
	var count int
	for _, row := range rows {
		numerator, ok := parseFloat(row[numeratorKey])
		if !ok {
			continue
		}
		denominator, ok := parseFloat(row[denominatorKey])
		if !ok || denominator == 0 {
			continue
		}
		sum += numerator / denominator
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

func averageCapExMargin(incomeRows []map[string]any, balanceRows []map[string]any) (float64, bool) {
	var sum float64
	var count int
	limit := minInt(len(incomeRows), len(balanceRows)-1)
	for i := 0; i < limit; i++ {
		capex, ok := deriveCapEx(balanceRows[i:], incomeRows[i])
		if !ok {
			continue
		}
		revenue, ok := parseFloat(incomeRows[i]["sale_account"])
		if !ok || revenue <= 0 {
			continue
		}
		sum += capex / revenue
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

func averageNWCMargin(incomeRows []map[string]any, balanceRows []map[string]any) (float64, bool) {
	var sum float64
	var count int
	limit := minInt(len(incomeRows), len(balanceRows)-1)
	for i := 0; i < limit; i++ {
		nwc, ok := deriveChangeInNWC(balanceRows[i:])
		if !ok {
			continue
		}
		revenue, ok := parseFloat(incomeRows[i]["sale_account"])
		if !ok || revenue == 0 {
			continue
		}
		sum += nwc / revenue
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

func deriveBetaFromRows(stockRows []map[string]any, indexRows []map[string]any) (float64, int, bool) {
	stockReturns := dailyReturnSeries(stockRows, []string{"stck_clpr", "stck_prpr"})
	indexReturns := dailyReturnSeries(indexRows, []string{"bstp_nmix_prpr", "stck_clpr", "stck_prpr"})
	if len(stockReturns) == 0 || len(indexReturns) == 0 {
		return 0, 0, false
	}

	matchedStock := make([]float64, 0, minInt(len(stockReturns), len(indexReturns)))
	matchedIndex := make([]float64, 0, minInt(len(stockReturns), len(indexReturns)))
	for date, stockRet := range stockReturns {
		indexRet, ok := indexReturns[date]
		if !ok {
			continue
		}
		matchedStock = append(matchedStock, stockRet)
		matchedIndex = append(matchedIndex, indexRet)
	}
	if len(matchedStock) < 60 {
		return 0, len(matchedStock), false
	}

	meanStock := averageFloatSlice(matchedStock)
	meanIndex := averageFloatSlice(matchedIndex)
	var covariance float64
	var variance float64
	for i := range matchedStock {
		stockDelta := matchedStock[i] - meanStock
		indexDelta := matchedIndex[i] - meanIndex
		covariance += stockDelta * indexDelta
		variance += indexDelta * indexDelta
	}
	if variance == 0 {
		return 0, len(matchedStock), false
	}
	beta := covariance / variance
	if math.IsNaN(beta) || math.IsInf(beta, 0) {
		return 0, len(matchedStock), false
	}
	return beta, len(matchedStock), true
}

func dailyReturnSeries(rows []map[string]any, closeKeys []string) map[string]float64 {
	type point struct {
		Date  string
		Close float64
	}

	points := make([]point, 0, len(rows))
	for _, row := range rows {
		date := fieldStringMap(row, "stck_bsop_date")
		if date == "" {
			continue
		}
		closeValue, ok := firstFloat(row, closeKeys...)
		if !ok || closeValue <= 0 {
			continue
		}
		points = append(points, point{Date: date, Close: closeValue})
	}
	if len(points) < 2 {
		return nil
	}

	sort.Slice(points, func(i, j int) bool { return points[i].Date < points[j].Date })
	returns := make(map[string]float64, len(points)-1)
	for i := 1; i < len(points); i++ {
		prev := points[i-1].Close
		if prev <= 0 {
			continue
		}
		returns[points[i].Date] = (points[i].Close / prev) - 1
	}
	return returns
}

func firstFloat(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := parseFloat(row[key]); ok {
			return value, true
		}
	}
	return 0, false
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

func firstRowOrNil(rows []map[string]any) map[string]any {
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func findDCFInput(inputs []DCFInputValue, name string) DCFInputValue {
	for _, input := range inputs {
		if input.Name == name {
			return input
		}
	}
	return DCFInputValue{Name: name, Status: DCFInputMissing}
}

func envOptionalFloat(key string) *float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parsed, ok := parseFloat(value)
	if !ok {
		return nil
	}
	return &parsed
}

func envFloatOrDefault(key string, defaultValue float64) float64 {
	value := envOptionalFloat(key)
	if value == nil {
		return defaultValue
	}
	return *value
}

func percentToRatio(value float64) float64 {
	if math.Abs(value) > 1 {
		return value / 100
	}
	return value
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
