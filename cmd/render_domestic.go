package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/dcf"
	"github.com/fifi/internal/domesticstock"
)

func printMarketTimeSummary(resp *auth.RESTResponse) {
	row := firstRow(resp, "output1")
	if row == nil {
		return
	}

	dates := joinNonEmpty(", ",
		fieldString(row, "date1"),
		fieldString(row, "date2"),
		fieldString(row, "date3"),
		fieldString(row, "date4"),
		fieldString(row, "date5"),
	)

	printSummaryBlock("Market Time Summary", []string{
		formatSummaryLine("Today", fieldString(row, "today")),
		formatSummaryLine("Now", fieldString(row, "time")),
		formatSummaryLine("Open / Close", joinNonEmpty(" / ", fieldString(row, "s_time"), fieldString(row, "e_time"))),
		formatSummaryLine("Business Days", dates),
	})
}

func printIndexSummary(name string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock(name+" Summary", []string{
		formatSummaryLine("Current", firstNonEmpty(row, "bstp_nmix_prpr", "stck_prpr")),
		formatSummaryLine("Change", firstNonEmpty(row, "bstp_nmix_prdy_vrss", "prdy_vrss")),
		formatSummaryLine("Rate", firstNonEmpty(row, "bstp_nmix_prdy_ctrt", "prdy_ctrt")),
		formatSummaryLine("Open / High / Low", joinNonEmpty(" / ",
			firstNonEmpty(row, "bstp_nmix_oprc", "stck_oprc"),
			firstNonEmpty(row, "bstp_nmix_hgpr", "stck_hgpr"),
			firstNonEmpty(row, "bstp_nmix_lwpr", "stck_lwpr"),
		)),
		formatSummaryLine("Volume", firstNonEmpty(row, "acml_vol")),
		formatSummaryLine("Turnover", firstNonEmpty(row, "acml_tr_pbmn")),
	})
}

func printKOSPIActualPBRSummary(result *domesticstock.ActualPBRResult) {
	if result == nil {
		return
	}

	lines := []string{
		formatSummaryLine("Method", result.Method),
		formatSummaryLine("Weighted PBR", formatFloat(result.WeightedPBR)),
		formatSummaryLine("Target Coverage", formatPercent(result.TargetCoverage)),
		formatSummaryLine("Used Coverage", formatPercent(result.UsedCoverage)),
		formatSummaryLine("Raw Coverage", formatPercent(result.RawCoverage)),
		formatSummaryLine("Selected / Used / Skipped", fmt.Sprintf("%d / %d / %d", result.SelectedCount, result.UsedCount, result.SkippedCount)),
		formatSummaryLine("Cache Hit / API Call", fmt.Sprintf("%d / %d", result.CacheHitCount, result.APICallCount)),
		formatSummaryLine("Market Cap Used", formatFloat(result.UsedMarketCap)),
		formatSummaryLine("Book Value Used", formatFloat(result.AggregateBookValue)),
		formatSummaryLine("Business Date", result.BusinessDate),
		formatSummaryLine("Actual PBR Cache", result.ActualPBRCachePath),
		formatSummaryLine("Master Cache", result.MasterCachePath),
		formatSummaryLine("Master JSON", result.MasterJSONPath),
		formatSummaryLine("Master Load", result.MasterLoadTime.String()),
		formatSummaryLine("Cache Load", result.CacheLoadTime.String()),
		formatSummaryLine("Rate Limit Wait", result.RateLimitWaitTime.String()),
		formatSummaryLine("Price Fetch", result.PriceFetchTime.String()),
		formatSummaryLine("Cache Save", result.CacheSaveTime.String()),
		formatSummaryLine("Total Time", result.TotalDuration.String()),
	}

	for i, constituent := range result.Constituents {
		if i >= 5 {
			break
		}
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("Top %d", i+1),
			fmt.Sprintf("%s %s | cap=%s | pbr=%s | weight=%s | %s",
				constituent.Code,
				constituent.Name,
				formatFloat(constituent.MarketCap),
				formatFloat(constituent.PBR),
				formatPercent(constituent.Coverage),
				cacheLabel(constituent.CacheHit),
			),
		))
	}

	printSummaryBlock("KOSPI Actual PBR Summary", lines)
}

func printDCFReadinessSummary(result *domesticstock.DCFReadinessResult) {
	if result == nil {
		return
	}

	lines := []string{
		formatSummaryLine("Symbol", result.Symbol),
		formatSummaryLine("Division", result.Division),
		formatSummaryLine("Balance / Income Periods", fmt.Sprintf("%d / %d", result.BalancePeriods, result.IncomePeriods)),
		formatSummaryLine("Can Project FCF", formatBool(result.CanProjectFCF)),
		formatSummaryLine("Can Compute WACC", formatBool(result.CanComputeWACC)),
		formatSummaryLine("Can Compute EV", formatBool(result.CanComputeEnterpriseValue)),
		formatSummaryLine("Can Compute Target Price", formatBool(result.CanComputeTargetPrice)),
		formatSummaryLine("Missing For FCF", strings.Join(result.MissingForFCF, ", ")),
		formatSummaryLine("Missing For WACC", strings.Join(result.MissingForWACC, ", ")),
		formatSummaryLine("Missing For Target Price", strings.Join(result.MissingForTargetPrice, ", ")),
	}

	for _, input := range result.Inputs {
		value := string(input.Status)
		if input.HasValue {
			value = DCFInputStatusText(input)
		}
		if input.Note != "" {
			value += " | " + input.Note
		}
		lines = append(lines, formatSummaryLine(input.Name, value))
	}

	printSummaryBlock("DCF Readiness Summary", lines)
}

func printDCFValuationSummary(result *domesticstock.DCFValuationResult) {
	if result == nil || result.Valuation == nil {
		return
	}

	lines := []string{
		formatSummaryLine("Symbol", result.Symbol),
		formatSummaryLine("Revenue / EBIT", fmt.Sprintf("%s / %s", formatFloat(result.Financial.Revenue), formatFloat(result.Financial.EBIT))),
		formatSummaryLine("Current Price", formatFloat(result.CurrentPrice)),
		formatSummaryLine("Base FCF", formatFloat(result.Valuation.BaseFCF)),
		formatSummaryLine("Risk Free / Beta / MRP", fmt.Sprintf("%s / %s / %s",
			formatPercent(result.Market.RiskFreeRate),
			formatFloat(result.Market.Beta),
			formatPercent(result.Market.MarketPremium),
		)),
		formatSummaryLine("Cost Of Debt", formatPercent(result.Market.CostOfDebt)),
		formatSummaryLine("Equity / Debt Weight", fmt.Sprintf("%s / %s",
			formatPercent(result.Market.EquityWeight),
			formatPercent(result.Market.DebtWeight),
		)),
		formatSummaryLine("Cost Of Equity", formatPercent(result.Valuation.CostOfEquity)),
		formatSummaryLine("WACC", formatPercent(result.Valuation.WACC)),
		formatSummaryLine("Terminal Growth", formatPercent(result.Assumptions.TerminalGrowth)),
		formatSummaryLine("Enterprise Value", formatFloat(result.Valuation.EnterpriseValue)),
		formatSummaryLine("Equity Value", formatFloat(result.Valuation.EquityValue)),
		formatSummaryLine("Net Debt", formatFloat(result.Financial.NetDebt)),
		formatSummaryLine("Shares Out", formatFloat(result.Financial.SharesOut)),
		formatSummaryLine("Target Price Raw", formatFloat(result.Valuation.TargetPriceRaw)),
		formatSummaryLine("Target Price Scale", formatFloat(result.Valuation.TargetPriceScale)),
		formatSummaryLine("Target Price Unit", result.Valuation.TargetPriceUnit),
		formatSummaryLine("Target Price", formatFloat(result.Valuation.TargetPrice)),
		formatSummaryLine("Projection", fmt.Sprintf("growth=%s ebit=%s dna=%s capex=%s nwc=%s",
			formatPercent(result.Projection.RevenueGrowth),
			formatPercent(result.Projection.EBITMargin),
			formatPercent(result.Projection.DNAMargin),
			formatPercent(result.Projection.CapExMargin),
			formatPercent(result.Projection.NWCMargin),
		)),
	}

	for i, year := range result.Valuation.Forecast {
		if i >= 3 {
			break
		}
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("Year %d", year.Year),
			fmt.Sprintf("rev=%s fcf=%s pv=%s",
				formatFloat(year.Revenue),
				formatFloat(year.FCF),
				formatFloat(year.PresentValue),
			),
		))
	}

	for i, note := range result.Notes {
		if i >= 3 {
			break
		}
		lines = append(lines, formatSummaryLine(fmt.Sprintf("Note %d", i+1), note))
	}

	printSummaryBlock("DCF Valuation Summary", lines)
}

func printReverseDCFSummary(result *dcf.ReverseDCFResult) {
	if result == nil || result.Valuation == nil {
		return
	}

	printSummaryBlock("Reverse DCF Summary", []string{
		formatSummaryLine("Target Price", formatFloat(result.TargetPrice)),
		formatSummaryLine("Implied Revenue Growth", formatPercent(result.ImpliedRevenueGrowth)),
		formatSummaryLine("Iterations", fmt.Sprintf("%d", result.Iterations)),
		formatSummaryLine("Price Error", formatFloat(result.PriceError)),
		formatSummaryLine("Solved WACC", formatPercent(result.Valuation.WACC)),
		formatSummaryLine("Solved EV / Equity", fmt.Sprintf("%s / %s",
			formatFloat(result.Valuation.EnterpriseValue),
			formatFloat(result.Valuation.EquityValue),
		)),
	})
}

func printMonteCarloSummary(result *dcf.MonteCarloResult, jsonPath string) {
	if result == nil {
		return
	}

	printSummaryBlock("Monte Carlo DCF Summary", []string{
		formatSummaryLine("Requested / Valid / Invalid", fmt.Sprintf("%d / %d / %d", result.RequestedIterations, result.ValidIterations, result.InvalidIterations)),
		formatSummaryLine("Mean", formatFloat(result.Mean)),
		formatSummaryLine("P10 / P50 / P90", fmt.Sprintf("%s / %s / %s",
			formatFloat(result.P10),
			formatFloat(result.P50),
			formatFloat(result.P90),
		)),
		formatSummaryLine("Min / Max", fmt.Sprintf("%s / %s",
			formatFloat(result.Min),
			formatFloat(result.Max),
		)),
		formatSummaryLine("JSON Export", jsonPath),
	})
}

func printProgramTradeSummary(resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock("Program Trade Summary", []string{
		formatSummaryLine("Date", firstNonEmpty(row, "stck_bsop_date")),
		formatSummaryLine("Market Close", humanNumber(firstNonEmpty(row, "stck_clpr"))),
		formatSummaryLine("Program Net Position", netFlowText(firstNonEmpty(row, "whol_smtn_ntby_qty"), "")),
		formatSummaryLine("Program Net Amount", netFlowText(firstNonEmpty(row, "whol_smtn_ntby_tr_pbmn"), "")),
		formatSummaryLine("Arbitrage Net Amount", netFlowText(firstNonEmpty(row, "prsm_nslg_pbmn"), "")),
		formatSummaryLine("Non-Arbitrage Net Amount", netFlowText(firstNonEmpty(row, "nprsm_nslg_pbmn"), "")),
		formatSummaryLine("Buy / Sell Volume", joinNonEmpty(" / ",
			humanNumber(firstNonEmpty(row, "whol_smtn_shnu_vol")),
			humanNumber(firstNonEmpty(row, "whol_smtn_seln_vol")),
		)),
	})
}

func printVISummary(resp *auth.RESTResponse) {
	rows := rowsFromResponse(resp, "output")
	if len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Triggered Items", fmt.Sprintf("%d", len(rows))),
	}

	limit := 3
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		row := rows[i]
		item := fmt.Sprintf(
			"%s (%s) | VI=%s | %s~%s",
			firstNonEmpty(row, "hts_kor_isnm"),
			firstNonEmpty(row, "mksc_shrn_iscd"),
			firstNonEmpty(row, "vi_cls_code"),
			firstNonEmpty(row, "cntg_vi_hour"),
			firstNonEmpty(row, "vi_cncl_hour"),
		)
		lines = append(lines, formatSummaryLine(fmt.Sprintf("Item %d", i+1), item))
	}

	printSummaryBlock("VI Summary", lines)
}

func printMarketFundsSummary(resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock("Market Funds Summary", []string{
		formatSummaryLine("Business Date", firstNonEmpty(row, "bsop_date")),
		formatSummaryLine("Customer Deposit", firstNonEmpty(row, "cust_dpmn_amt")),
		formatSummaryLine("Deposit Change", firstNonEmpty(row, "cust_dpmn_amt_prdy_vrss")),
		formatSummaryLine("Credit Loan Balance", firstNonEmpty(row, "crdt_loan_rmnd")),
		formatSummaryLine("Futures Deposit", firstNonEmpty(row, "futs_tfam_amt")),
		formatSummaryLine("Amount Turnover", firstNonEmpty(row, "amt_tnrt")),
	})
}

func printClientMetricsSummary(metrics auth.HTTPMetricsSnapshot) {
	lines := []string{
		formatSummaryLine("Call Count", strconv.Itoa(metrics.CallCount)),
		formatSummaryLine("Success / Error", fmt.Sprintf("%d / %d", metrics.SuccessCount, metrics.ErrorCount)),
		formatSummaryLine("Total Time", metrics.TotalDuration.String()),
		formatSummaryLine("Average Time", metrics.AverageTime.String()),
		formatSummaryLine("Elapsed", metrics.Elapsed.String()),
		formatSummaryLine("RPM", formatFloat(metrics.RPM)),
	}

	if !metrics.StartedAt.IsZero() {
		lines = append(lines, formatSummaryLine("Started At", metrics.StartedAt.Format(time.RFC3339)))
	}
	if !metrics.LastCallAt.IsZero() {
		lines = append(lines, formatSummaryLine("Last Call At", metrics.LastCallAt.Format(time.RFC3339)))
	}

	printSummaryBlock("KIClient Metrics", lines)
}
