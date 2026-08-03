package snapshot

import (
	"fmt"
	"strings"

	"github.com/fifi/internal/format"
)

func pp(v float64) string { return format.Signed(v, 1) + "%p" }

func na(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "manual input not provided"
	}
	return "n/a (" + reason + ")"
}

func valuePercent(value *float64, reason string) string {
	if value == nil {
		return na(reason)
	}
	return format.PercentPlain(*value)
}

func quotePercent(value *float64, reason string) string {
	if value == nil {
		return na(reason)
	}
	return format.Percent(*value)
}

func appendReason(value string, reason string) string {
	if strings.TrimSpace(reason) == "" {
		return value
	}
	return fmt.Sprintf("%s (%s)", value, reason)
}

// Package-private delegates to formatting package
func number(v float64, decimals int) string { return format.Number(v, decimals) }
func signedNumber(v float64, decimals int) string { return format.Signed(v, decimals) }
func percent(v float64) string { return format.Percent(v) }
func percentPlain(v float64) string { return format.PercentPlain(v) }
func eok(v float64) string { return format.Eok(v) }
func eokPlain(v float64) string { return format.EokPlain(v) }
func trillionFromEok(v float64) string { return format.TrillionFromEok(v) }
func trillionFromEokPlain(v float64) string { return format.TrillionFromEokPlain(v) }

