package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/commodityfuture"
	"github.com/fifi/internal/shippingindex"
)

func printCommodityFutureSummary(name string, quote *commodityfuture.Quote) {
	if quote == nil {
		return
	}

	printSummaryBlock(name+" Summary", []string{
		formatSummaryLine("Provider", commodityProviderPathText(quote.ProviderPath)),
		formatSummaryLine("Symbol / Product", joinNonEmpty(" / ", quote.Symbol, quote.ProductCode)),
		formatSummaryLine("Contract", joinNonEmpty(" / ", quote.Contract, quote.QuoteCode)),
		formatSummaryLine("Current", quoteNumberText(quote.Price)),
		formatSummaryLine("Change / Rate", joinNonEmpty(" / ",
			quoteSignedNumberText(quote.Change),
			quotePercentText(quote.ChangePercent),
		)),
		formatSummaryLine("Previous Close", quoteNumberText(quote.PreviousClose)),
		formatSummaryLine("Open / High / Low", joinNonEmpty(" / ",
			quoteNumberText(quote.Open),
			quoteNumberText(quote.High),
			quoteNumberText(quote.Low),
		)),
		formatSummaryLine("Volume", quoteNumberText(quote.Volume)),
		formatSummaryLine("Exchange / Currency", joinNonEmpty(" / ", quote.Exchange, quote.Currency)),
		formatSummaryLine("As Of", quote.AsOf),
		formatSummaryLine("Delay", quote.Delay),
		formatSummaryLine("Source", quote.SourceURL),
		formatSummaryLine("CME Page", quote.ReferenceURL),
		formatSummaryLine("Note", quote.Note),
	})
}

func printOverseasStockSummary(name string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock(name+" Summary", []string{
		formatSummaryLine("Symbol", firstNonEmpty(row, "symb", "rsym")),
		formatSummaryLine("Current", firstNonEmpty(row, "last")),
		formatSummaryLine("Change", firstNonEmpty(row, "diff")),
		formatSummaryLine("Rate", firstNonEmpty(row, "rate")),
		formatSummaryLine("Previous Close", firstNonEmpty(row, "base")),
		formatSummaryLine("Volume", firstNonEmpty(row, "tvol")),
		formatSummaryLine("Turnover", firstNonEmpty(row, "tamt")),
	})
}

func printExchangeRateSummary(marketDivCode string, symbol string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output1")
	if row == nil {
		return
	}

	printSummaryBlock("Exchange Rate Summary", []string{
		formatSummaryLine("Market / Symbol", joinNonEmpty(" / ", marketDivCode, symbol)),
		formatSummaryLine("Name", firstNonEmpty(row, "hts_kor_isnm")),
		formatSummaryLine("Current", firstNonEmpty(row, "ovrs_nmix_prpr")),
		formatSummaryLine("Change", firstNonEmpty(row, "ovrs_nmix_prdy_vrss")),
		formatSummaryLine("Rate", firstNonEmpty(row, "prdy_ctrt")),
		formatSummaryLine("Previous Close", firstNonEmpty(row, "ovrs_nmix_prdy_clpr")),
		formatSummaryLine("Open / High / Low", joinNonEmpty(" / ",
			firstNonEmpty(row, "ovrs_nmix_oprc", "ovrs_prod_oprc"),
			firstNonEmpty(row, "ovrs_nmix_hgpr", "ovrs_prod_hgpr"),
			firstNonEmpty(row, "ovrs_nmix_lwpr", "ovrs_prod_lwpr"),
		)),
		formatSummaryLine("Volume", firstNonEmpty(row, "acml_vol")),
	})
}

func printShippingIndexSummary(quotes []shippingindex.Quote) {
	if len(quotes) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Source", "StockQ public market summary"),
	}

	for _, quote := range quotes {
		lines = append(lines, formatSummaryLine(
			quote.Symbol,
			fmt.Sprintf("%s (%s) | %s / %s / %s | local %s",
				quote.Name,
				quote.Symbol,
				humanNumber(strconv.FormatFloat(quote.Price, 'f', -1, 64)),
				formatSignedFloat(quote.Change),
				formatPercentPoints(quote.ChangePercent),
				quote.Local,
			),
		))
	}

	printSummaryBlock("Shipping Index Summary", lines)
}

func configuredCommodityInstrument(name string, ticker string, productCode string, cmeProductID string) commodityfuture.Instrument {
	instrument := commodityfuture.ResolveInstrument(name, ticker, productCode)
	if strings.TrimSpace(cmeProductID) != "" {
		instrument.CMEProductID = strings.TrimSpace(cmeProductID)
	}
	return instrument
}
