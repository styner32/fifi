package main

import (
	"fmt"
	"log"

	"github.com/fifi/internal/companyanalysis"
	"github.com/fifi/internal/dcf"
)

func printCompanyAnalysisSummary(result *companyanalysis.Result) {
	if result == nil || result.Valuation == nil {
		log.Printf("company analysis summary: result or valuation is nil: %v", result)
		return
	}

	upside := ""
	if result.Quote.Price > 0 {
		upside = formatPercent((result.Valuation.TargetPrice / result.Quote.Price) - 1)
	}

	lines := []string{
		formatSummaryLine("Company / Symbol", joinNonEmpty(" / ", result.CompanyName, result.Symbol)),
		formatSummaryLine("CIK / Benchmark", joinNonEmpty(" / ", result.CIK, result.BenchmarkSymbol)),
		formatSummaryLine("Quote", joinNonEmpty(" ", formatMoney(result.Quote.Currency, result.Quote.Price), "on", result.Quote.PriceDate)),
		formatSummaryLine("Previous / Change", fmt.Sprintf("%s / %s (%s)",
			formatMoney(result.Quote.Currency, result.Quote.PreviousClose),
			formatSignedMoney(result.Quote.Currency, result.Quote.Change),
			formatPercentPoints(result.Quote.ChangePercent),
		)),
		formatSummaryLine("Revenue / EBIT / Net Income", fmt.Sprintf("%s / %s / %s",
			formatMoney(result.Quote.Currency, result.Financials[0].Revenue),
			formatMoney(result.Quote.Currency, result.Financials[0].EBIT),
			formatMoney(result.Quote.Currency, result.Financials[0].NetIncome),
		)),
		formatSummaryLine("Cash / Debt / Net Debt", fmt.Sprintf("%s / %s / %s",
			formatMoney(result.Quote.Currency, result.Financials[0].Cash),
			formatMoney(result.Quote.Currency, result.Financials[0].TotalDebt),
			formatMoney(result.Quote.Currency, result.KeyMetrics.NetDebt),
		)),
		formatSummaryLine("Market Cap / EV", fmt.Sprintf("%s / %s",
			formatMoney(result.Quote.Currency, result.KeyMetrics.MarketCap),
			formatMoney(result.Quote.Currency, result.KeyMetrics.EnterpriseValue),
		)),
		formatSummaryLine("Growth / Op Margin / ROE", fmt.Sprintf("%s / %s / %s",
			formatPercent(result.KeyMetrics.RevenueGrowth),
			formatPercent(result.KeyMetrics.OperatingMargin),
			formatPercent(result.KeyMetrics.ROE),
		)),
		formatSummaryLine("Current Ratio / Debt To Equity", fmt.Sprintf("%s / %s",
			formatFloat(result.KeyMetrics.CurrentRatio),
			formatFloat(result.KeyMetrics.DebtToEquity),
		)),
		formatSummaryLine("Risk Free / Beta / MRP", fmt.Sprintf("%s / %s / %s",
			formatPercent(result.Market.RiskFreeRate),
			formatFloat(result.Market.Beta),
			formatPercent(result.Market.MarketPremium),
		)),
		formatSummaryLine("Cost Of Debt / WACC", fmt.Sprintf("%s / %s",
			formatPercent(result.Market.CostOfDebt),
			formatPercent(result.Valuation.WACC),
		)),
		formatSummaryLine("Target Price", joinNonEmpty(" ", formatMoney(result.Quote.Currency, result.Valuation.TargetPrice), "("+result.Valuation.TargetPriceUnit+")")),
		formatSummaryLine("Upside / Downside", upside),
		formatSummaryLine("Projection", fmt.Sprintf("growth=%s ebit=%s dna=%s capex=%s nwc=%s",
			formatPercent(result.Projection.RevenueGrowth),
			formatPercent(result.Projection.EBITMargin),
			formatPercent(result.Projection.DNAMargin),
			formatPercent(result.Projection.CapExMargin),
			formatPercent(result.Projection.NWCMargin),
		)),
	}

	for i, record := range result.Financials {
		if i >= 3 {
			break
		}
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("FY %d", record.FiscalYear),
			fmt.Sprintf("rev=%s ebit=%s cash=%s debt=%s",
				formatMoney(result.Quote.Currency, record.Revenue),
				formatMoney(result.Quote.Currency, record.EBIT),
				formatMoney(result.Quote.Currency, record.Cash),
				formatMoney(result.Quote.Currency, record.TotalDebt),
			),
		))
	}

	for i, note := range result.Notes {
		if i >= 3 {
			break
		}
		lines = append(lines, formatSummaryLine(fmt.Sprintf("Note %d", i+1), note))
	}

	printSummaryBlock("Company Analysis Summary", lines)
}

func printCompanyReverseDCFSummary(result *companyanalysis.Result, reverse *dcf.ReverseDCFResult) {
	if result == nil || reverse == nil || reverse.Valuation == nil {
		return
	}

	printSummaryBlock("Company Reverse DCF Summary", []string{
		formatSummaryLine("Market Price", formatMoney(result.Quote.Currency, result.Quote.Price)),
		formatSummaryLine("Implied Revenue Growth", formatPercent(reverse.ImpliedRevenueGrowth)),
		formatSummaryLine("Iterations", fmt.Sprintf("%d", reverse.Iterations)),
		formatSummaryLine("Price Error", formatMoney(result.Quote.Currency, reverse.PriceError)),
		formatSummaryLine("Solved WACC", formatPercent(reverse.Valuation.WACC)),
	})
}

func printCompanyMonteCarloSummary(result *dcf.MonteCarloResult, jsonPath string) {
	if result == nil {
		return
	}

	printSummaryBlock("Company Monte Carlo DCF Summary", []string{
		formatSummaryLine("Requested / Valid / Invalid", fmt.Sprintf("%d / %d / %d", result.RequestedIterations, result.ValidIterations, result.InvalidIterations)),
		formatSummaryLine("Mean", formatFloat(result.Mean)),
		formatSummaryLine("P10 / P50 / P90", fmt.Sprintf("%s / %s / %s",
			formatFloat(result.P10),
			formatFloat(result.P50),
			formatFloat(result.P90),
		)),
		formatSummaryLine("Min / Max", fmt.Sprintf("%s / %s", formatFloat(result.Min), formatFloat(result.Max))),
		formatSummaryLine("JSON Export", jsonPath),
	})
}
