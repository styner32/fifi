package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/commodityfuture"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/quadwitching"
)

func printQuadWitchingGateSummary(window quadwitching.RunWindow, forceRun bool, shouldRun bool) {
	status := "skip"
	if shouldRun {
		status = "run"
	}

	reason := "outside run window"
	if forceRun {
		reason = "forced by QUAD_WITCHING_FORCE"
	} else if window.ShouldRun {
		reason = "inside quad witching window"
	}

	printSummaryBlock("Quad Witching Gate", []string{
		formatSummaryLine("Business Date", window.BusinessDate),
		formatSummaryLine("Target Date", window.QuadDate),
		formatSummaryLine("Window", joinNonEmpty(" ~ ", window.WindowStart, window.WindowEnd)),
		formatSummaryLine("Lookahead / Grace", fmt.Sprintf("%d / %d day(s)", window.LookaheadDays, window.GraceDays)),
		formatSummaryLine("Days Until Target", strconv.Itoa(window.DaysUntil)),
		formatSummaryLine("Status", status),
		formatSummaryLine("Reason", reason),
	})
}

func printQuadWitchingContractSummary(resolved *domesticfutureoption.ResolvedContract) {
	if resolved == nil {
		return
	}

	printSummaryBlock("Quad Witching Contract", []string{
		formatSummaryLine("Business Date", resolved.BusinessDate),
		formatSummaryLine("Source", resolved.Source),
		formatSummaryLine("Contract", joinNonEmpty(" ", resolved.Record.ShortCode, resolved.Record.Name)),
		formatSummaryLine("Month Class", resolved.Record.MonthClassCode),
		formatSummaryLine("Underlying", joinNonEmpty(" / ", resolved.Record.UnderlyingShortCode, resolved.Record.UnderlyingName)),
		formatSummaryLine("Master Cache", resolved.MasterCachePath),
		formatSummaryLine("Master JSON", resolved.MasterJSONPath),
	})
}

func printFuturePriceSummary(resp *auth.RESTResponse) {
	row := firstRowAny(resp, "output1", "output", "output2", "output3")
	if row == nil {
		return
	}

	printSummaryBlock("Quad Witching Futures Price", []string{
		formatSummaryLine("Contract", firstNonEmpty(row, "hts_kor_isnm")),
		formatSummaryLine("Futures Price", humanNumber(firstNonEmpty(row, "futs_prpr"))),
		formatSummaryLine("Spot Index", humanNumber(firstNonEmpty(row, "bstp_nmix_prpr"))),
		formatSummaryLine("Basis (Futures - Spot)", formatOptionalHumanNumber(computeBasis(row))),
		formatSummaryLine("Market Basis", humanNumber(firstNonEmpty(row, "mrkt_basis"))),
		formatSummaryLine("Open Interest", joinNonEmpty(" / ",
			humanNumber(firstNonEmpty(row, "hts_otst_stpl_qty")),
			signedChangeText(firstNonEmpty(row, "otst_stpl_qty_icdc"), "increase", "decrease", ""),
		)),
		formatSummaryLine("Days To Expiry", daysText(firstNonEmpty(row, "hts_rmnn_dynu"))),
		formatSummaryLine("Volume / Turnover", joinNonEmpty(" / ",
			humanNumber(firstNonEmpty(row, "acml_vol")),
			humanNumber(firstNonEmpty(row, "acml_tr_pbmn")),
		)),
	})
}

func printFutureBoardTopSummary(resp *auth.RESTResponse) {
	topRow := firstRow(resp, "output1")
	if topRow == nil {
		return
	}

	printSummaryBlock("Quad Witching Futures Board Top", []string{
		formatSummaryLine("Underlying Price", humanNumber(firstNonEmpty(topRow, "unas_prpr"))),
		formatSummaryLine("Underlying Change / Rate", joinNonEmpty(" / ",
			signedChangeText(firstNonEmpty(topRow, "unas_prdy_vrss"), "up", "down", ""),
			firstNonEmpty(topRow, "unas_prdy_ctrt"),
		)),
		formatSummaryLine("Futures Price", humanNumber(firstNonEmpty(topRow, "futs_prpr"))),
		formatSummaryLine("Futures Change / Rate", joinNonEmpty(" / ",
			signedChangeText(firstNonEmpty(topRow, "futs_prdy_vrss"), "up", "down", ""),
			firstNonEmpty(topRow, "futs_prdy_ctrt"),
		)),
		formatSummaryLine("Days To Expiry", daysText(firstNonEmpty(topRow, "hts_rmnn_dynu"))),
	})
}

func printFutureBoardSummary(resp *auth.RESTResponse, targetCode string) {
	rows := rowsFromResponse(resp, "output")
	if len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Contracts", fmt.Sprintf("%d", len(rows))),
	}

	if target := findRowByValue(rows, "futs_shrn_iscd", targetCode); target != nil {
		lines = append(lines, formatSummaryLine(
			"Selected Contract",
			fmt.Sprintf("%s %s, current %s, open interest %s, %s to expiry, expected match %s",
				firstNonEmpty(target, "futs_shrn_iscd"),
				firstNonEmpty(target, "hts_kor_isnm"),
				humanNumber(firstNonEmpty(target, "futs_prpr")),
				humanNumber(firstNonEmpty(target, "hts_otst_stpl_qty")),
				daysText(firstNonEmpty(target, "hts_rmnn_dynu")),
				humanNumber(firstNonEmpty(target, "futs_antc_cnpr")),
			),
		))
	}

	limit := 3
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		row := rows[i]
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("Board Row %d", i+1),
			fmt.Sprintf("%s %s, current %s, open interest %s, expected match %s",
				firstNonEmpty(row, "futs_shrn_iscd"),
				firstNonEmpty(row, "hts_kor_isnm"),
				humanNumber(firstNonEmpty(row, "futs_prpr")),
				humanNumber(firstNonEmpty(row, "hts_otst_stpl_qty")),
				humanNumber(firstNonEmpty(row, "futs_antc_cnpr")),
			),
		))
	}

	printSummaryBlock("Quad Witching Futures Board", lines)
}

func printFutureTimeChartSummary(resp *auth.RESTResponse) {
	header := firstRow(resp, "output1")
	rows := rowsFromResponse(resp, "output2")

	lines := []string{
		formatSummaryLine("Contract", firstNonEmpty(header, "hts_kor_isnm")),
		formatSummaryLine("Samples", fmt.Sprintf("%d", len(rows))),
	}

	if len(rows) > 0 {
		latest := rows[0]
		lines = append(lines,
			formatSummaryLine("Latest Time", firstNonEmpty(latest, "stck_cntg_hour")),
			formatSummaryLine("Latest Futures Price", humanNumber(firstNonEmpty(latest, "futs_prpr"))),
			formatSummaryLine("Latest Basis / KOSPI200", joinNonEmpty(" / ",
				humanNumber(firstNonEmpty(latest, "basis")),
				humanNumber(firstNonEmpty(latest, "kospi200_nmix")),
			)),
			formatSummaryLine("Latest Trade Volume", humanNumber(firstNonEmpty(latest, "cntg_vol"))),
			formatSummaryLine("Open Interest / Change", joinNonEmpty(" / ",
				humanNumber(firstNonEmpty(latest, "hts_otst_stpl_qty")),
				signedChangeText(firstNonEmpty(latest, "otst_stpl_qty_icdc"), "increase", "decrease", ""),
			)),
		)
	}

	printSummaryBlock("Quad Witching Futures Time Chart", lines)
}

func printFutureExpectedPriceSummary(resp *auth.RESTResponse) {
	header := firstRow(resp, "output1")
	rows := rowsFromResponse(resp, "output2")
	if header == nil && len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Contract", firstNonEmpty(header, "hts_kor_isnm")),
		formatSummaryLine("Samples", fmt.Sprintf("%d", len(rows))),
	}

	if len(rows) > 0 {
		latest := rows[0]
		lines = append(lines,
			formatSummaryLine("Latest Time", firstNonEmpty(latest, "stck_cntg_hour")),
			formatSummaryLine("Expected Match Price", humanNumber(firstNonEmpty(latest, "futs_antc_cnpr"))),
			formatSummaryLine("Expected Change", signedChangeText(firstNonEmpty(latest, "futs_antc_cntg_vrss"), "up", "down", "")),
			formatSummaryLine("Expected Rate", firstNonEmpty(latest, "antc_cntg_prdy_ctrt")),
		)
	}

	printSummaryBlock("Quad Witching Expected Price", lines)
}

func printFutureExecutionSummary(resp *auth.RESTResponse) {
	row := firstRowAny(resp, "output", "output1", "output2")
	rows := rowsFromResponse(resp, "output")
	if row == nil && len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Rows", fmt.Sprintf("%d", len(rows))),
	}
	if row != nil {
		lines = append(lines,
			formatSummaryLine("Latest Time", firstNonEmpty(row, "stck_cntg_hour", "aspr_acpt_hour")),
			formatSummaryLine("Latest Price / Change", joinNonEmpty(" / ",
				humanNumber(firstNonEmpty(row, "futs_prpr")),
				signedChangeText(firstNonEmpty(row, "futs_prdy_vrss", "futs_antc_cntg_vrss"), "up", "down", ""),
			)),
			formatSummaryLine("Trade Volume", humanNumber(firstNonEmpty(row, "cntg_vol", "acml_vol"))),
			formatSummaryLine("First Row", preview(row)),
		)
	}

	printSummaryBlock("Quad Witching Futures Execution", lines)
}

func printFutureMemberSummary(resp *auth.RESTResponse) {
	row := firstRowAny(resp, "output", "output1", "output2")
	rows := rowsFromResponse(resp, "output")
	if row == nil && len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Rows", fmt.Sprintf("%d", len(rows))),
	}
	if row != nil {
		lines = append(lines,
			formatSummaryLine("Foreign Position", netFlowText(firstNonEmpty(row, "frgn_ntby_qty", "frgn_shnu_vol", "frgn_seln_vol"), "계약")),
			formatSummaryLine("Institution Position", netFlowText(firstNonEmpty(row, "orgn_ntby_qty", "orgn_shnu_vol", "orgn_seln_vol"), "계약")),
			formatSummaryLine("Personal Position", netFlowText(firstNonEmpty(row, "prsn_ntby_qty", "prsn_shnu_vol", "prsn_seln_vol"), "계약")),
			formatSummaryLine("Raw First Row", preview(row)),
		)
	}

	printSummaryBlock("Quad Witching Futures Member", lines)
}

func printInvestorTrendSummary(title string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	if strings.TrimSpace(title) == "" {
		title = "Quad Witching Investor Trend"
	}

	printSummaryBlock(title, []string{
		formatSummaryLine("Business Date", firstNonEmpty(row, "stck_bsop_date")),
		formatSummaryLine("Personal", netFlowText(firstNonEmpty(row, "prsn_ntby_qty"), "주")),
		formatSummaryLine("Foreign", netFlowText(firstNonEmpty(row, "frgn_ntby_qty"), "주")),
		formatSummaryLine("Institution", netFlowText(firstNonEmpty(row, "orgn_ntby_qty"), "주")),
		formatSummaryLine("Net Amounts", joinNonEmpty(" / ",
			"개인 "+netFlowText(firstNonEmpty(row, "prsn_ntby_tr_pbmn"), ""),
			"외국인 "+netFlowText(firstNonEmpty(row, "frgn_ntby_tr_pbmn"), ""),
			"기관 "+netFlowText(firstNonEmpty(row, "orgn_ntby_tr_pbmn"), ""),
		)),
	})
}

func printForeignInstitutionSummary(resp *auth.RESTResponse) {
	rows := rowsFromResponse(resp, "output")
	if len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Rows", fmt.Sprintf("%d", len(rows))),
	}

	limit := 30
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		row := rows[i]
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("Rank %d", i+1),
			fmt.Sprintf("%s %s, 현재가 %s, 순수량은 %s, 순대금은 %s",
				firstNonEmpty(row, "mksc_shrn_iscd"),
				firstNonEmpty(row, "hts_kor_isnm"),
				stockPriceText(firstNonEmpty(row, "stck_prpr")),
				netFlowText(firstNonEmpty(row, "ntby_qty", "frgn_ntby_qty", "orgn_ntby_qty"), "주"),
				netFlowText(firstNonEmpty(row, "frgn_ntby_tr_pbmn", "orgn_ntby_tr_pbmn"), ""),
			),
		))
	}

	printSummaryBlock("Quad Witching Foreign/Institution Total", lines)
}

func printAskingPriceExpSummary(symbol string, resp *auth.RESTResponse) {
	orderBook := firstRow(resp, "output1")
	expected := firstRowAny(resp, "output2", "output")
	if orderBook == nil && expected == nil {
		return
	}

	printSummaryBlock("Quad Witching Closing Auction", []string{
		formatSummaryLine("Symbol", symbol),
		formatSummaryLine("Current Price", stockPriceText(firstNonEmpty(orderBook, "stck_prpr"))),
		formatSummaryLine("Expected Match Price", stockPriceText(firstNonEmpty(expected, "antc_cnpr"))),
		formatSummaryLine("Expected Change / Rate", joinNonEmpty(" / ",
			signedChangeText(firstNonEmpty(expected, "antc_cntg_vrss"), "up", "down", "flat"),
			firstNonEmpty(expected, "antc_cntg_prdy_ctrt"),
		)),
		formatSummaryLine("Expected Match Volume", humanNumber(firstNonEmpty(expected, "antc_vol"))),
		formatSummaryLine("Total Ask / Bid Balance", joinNonEmpty(" / ",
			humanNumber(firstNonEmpty(orderBook, "total_askp_rsqn")),
			humanNumber(firstNonEmpty(orderBook, "total_bidp_rsqn")),
		)),
		formatSummaryLine("VI Code", firstNonEmpty(orderBook, "vi_cls_code")),
	})
}

func printQuadWitchingSnapshotSummary(path string, snapshot quadwitching.SnapshotExport) {
	printSummaryBlock("Quad Witching Snapshot", []string{
		formatSummaryLine("Business Date", snapshot.BusinessDate),
		formatSummaryLine("Futures", joinNonEmpty(" ", snapshot.FuturesCode, snapshot.FuturesName)),
		formatSummaryLine("Endpoint Count", fmt.Sprintf("%d", len(snapshot.EndpointStates))),
		formatSummaryLine("JSON Export", path),
	})
}

func printConfiguredCommodityFutureSummary(
	ctx context.Context,
	svc *commodityfuture.Service,
	instrument commodityfuture.Instrument,
) {
	quote, err := svc.Quote(ctx, instrument)
	if err != nil {
		log.Printf("%s future quote error: %v", instrument.Name, err)
		return
	}

	title := joinNonEmpty(" ", instrument.Name, instrument.Symbol)
	printCommodityFutureSummary(title, quote)
}
