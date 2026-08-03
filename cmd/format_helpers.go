package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fifi/internal/commodityfuture"
	"github.com/fifi/internal/domesticstock"
)

func humanNumber(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return ""
	}

	parsedInt, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return formatIntWithCommas(parsedInt)
	}

	parsedFloat, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}

	if parsedFloat == float64(int64(parsedFloat)) {
		return formatIntWithCommas(int64(parsedFloat))
	}

	negative := parsedFloat < 0
	if negative {
		parsedFloat = -parsedFloat
	}
	raw := strconv.FormatFloat(parsedFloat, 'f', 2, 64)
	parts := strings.SplitN(raw, ".", 2)
	intPart, _ := strconv.ParseInt(parts[0], 10, 64)
	formatted := formatIntWithCommas(intPart)
	if len(parts) == 2 {
		decimal := strings.TrimRight(parts[1], "0")
		if decimal != "" {
			formatted += "." + decimal
		}
	}
	if negative {
		return "-" + formatted
	}
	return formatted
}

func formatIntWithCommas(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}

	raw := strconv.FormatInt(value, 10)
	if len(raw) <= 3 {
		if negative {
			return "-" + raw
		}
		return raw
	}

	var builder strings.Builder
	if negative {
		builder.WriteByte('-')
	}
	prefixLen := len(raw) % 3
	if prefixLen == 0 {
		prefixLen = 3
	}
	builder.WriteString(raw[:prefixLen])
	for i := prefixLen; i < len(raw); i += 3 {
		builder.WriteByte(',')
		builder.WriteString(raw[i : i+3])
	}
	return builder.String()
}

func netFlowText(value string, unit string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil {
		return joinNonEmpty(" ", value, unit)
	}

	action := "순매수"
	if parsed < 0 {
		action = "순매도"
		parsed = -parsed
	}
	if parsed == 0 {
		action = "중립"
	}

	number := humanNumber(strconv.FormatFloat(parsed, 'f', -1, 64))
	if action == "중립" {
		if unit != "" {
			return action + " " + number + unit
		}
		return action + " " + number
	}
	return joinNonEmpty(" ", action, number+unit)
}

func signedChangeText(value string, positiveWord string, negativeWord string, zeroWord string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil {
		return value
	}

	switch {
	case parsed > 0:
		if positiveWord == "" {
			return "+" + humanNumber(strconv.FormatFloat(parsed, 'f', -1, 64))
		}
		return positiveWord + " " + humanNumber(strconv.FormatFloat(parsed, 'f', -1, 64))
	case parsed < 0:
		if negativeWord == "" {
			return humanNumber(strconv.FormatFloat(parsed, 'f', -1, 64))
		}
		return negativeWord + " " + humanNumber(strconv.FormatFloat(-parsed, 'f', -1, 64))
	default:
		if zeroWord != "" {
			return zeroWord
		}
		return "0"
	}
}

func stockPriceText(value string) string {
	value = humanNumber(value)
	if value == "" {
		return ""
	}
	return value + "원"
}

func daysText(value string) string {
	value = humanNumber(value)
	if value == "" {
		return ""
	}
	return value + "일"
}

func DCFInputStatusText(input domesticstock.DCFInputValue) string {
	if !input.HasValue {
		return string(input.Status)
	}

	switch input.Name {
	case "EffectiveTax", "EquityWeight", "DebtWeight", "RiskFreeRate", "MarketPremium", "CostOfDebt", "RevenueGrowth", "EBITMargin", "DNAMargin", "CapExMargin", "NWCMargin":
		return fmt.Sprintf("%s (%s)", formatPercent(input.Value), input.Status)
	default:
		return fmt.Sprintf("%s (%s)", formatFloat(input.Value), input.Status)
	}
}

func formatBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func formatOptionalFloat(value float64, ok bool) string {
	if !ok {
		return ""
	}
	return formatFloat(value)
}

func formatOptionalHumanNumber(value float64, ok bool) string {
	if !ok {
		return ""
	}
	return humanNumber(strconv.FormatFloat(value, 'f', -1, 64))
}

func joinNonEmpty(sep string, parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, sep)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func formatFloat(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func formatSignedFloat(value float64) string {
	if value > 0 {
		return "+" + formatFloat(value)
	}
	return formatFloat(value)
}

func formatPercentPoints(value float64) string {
	return fmt.Sprintf("%.2f%%", value)
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.2f%%", value*100)
}

func formatMoney(currency string, value float64) string {
	label := strings.TrimSpace(currency)
	number := humanNumber(strconv.FormatFloat(value, 'f', 2, 64))
	if label == "" {
		return number
	}
	return label + " " + number
}

func formatSignedMoney(currency string, value float64) string {
	if value > 0 {
		return "+" + formatMoney(currency, value)
	}
	return formatMoney(currency, value)
}

func commodityProviderPathText(path string) string {
	parts := splitCSV(strings.ReplaceAll(path, "->", ","))
	if len(parts) == 0 {
		return ""
	}

	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case commodityfuture.ProviderCME:
			labels = append(labels, "CME delayed")
		case commodityfuture.ProviderYahoo:
			labels = append(labels, "Yahoo Finance")
		default:
			labels = append(labels, part)
		}
	}
	return strings.Join(labels, " -> ")
}

func quoteNumberText(value *float64) string {
	if value == nil {
		return ""
	}
	return humanNumber(strconv.FormatFloat(*value, 'f', -1, 64))
}

func quoteSignedNumberText(value *float64) string {
	if value == nil {
		return ""
	}
	number := quoteNumberText(value)
	if number == "" {
		return ""
	}
	if *value > 0 {
		return "+" + number
	}
	return number
}

func quotePercentText(value *float64) string {
	if value == nil {
		return ""
	}
	return formatPercentPoints(*value)
}

func cacheLabel(cacheHit bool) string {
	if cacheHit {
		return "cache"
	}
	return "api"
}
