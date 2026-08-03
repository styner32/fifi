package format

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Number formats float64 to grouped string with specified decimals.
func Number(v float64, decimals int) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = math.Abs(v)
	}
	raw := strconv.FormatFloat(v, 'f', decimals, 64)
	parts := strings.Split(raw, ".")
	intPart := parts[0]
	var grouped []byte
	for i, r := range reverse(intPart) {
		if i > 0 && i%3 == 0 {
			grouped = append(grouped, ',')
		}
		grouped = append(grouped, byte(r))
	}
	out := sign + reverse(string(grouped))
	if len(parts) == 2 {
		out += "." + parts[1]
	}
	return out
}

// Signed formats float64 with a leading plus sign for positive numbers.
func Signed(v float64, decimals int) string {
	if v > 0 {
		return "+" + Number(v, decimals)
	}
	return Number(v, decimals)
}

// Percent formats float64 as signed percentage with 2 decimals (e.g. "+1.23%").
func Percent(v float64) string {
	return Signed(v, 2) + "%"
}

// PercentPlain formats float64 as percentage with 2 decimals without leading plus.
func PercentPlain(v float64) string {
	return Number(v, 2) + "%"
}

// Eok formats float64 to Eok (round and display as signed integer).
func Eok(v float64) string {
	return Signed(math.Round(v), 0)
}

// TrillionFromEok formats Eok amount in trillions of KRW.
func TrillionFromEok(v float64) string {
	return Signed(v/10000, 1) + "조원"
}

// TrillionFromEokPlain formats Eok amount in trillions of KRW without leading sign.
func TrillionFromEokPlain(v float64) string {
	return Number(v/10000, 1) + "조원"
}

// EokPlain formats float64 to Eok (round and display as unsigned integer).
func EokPlain(v float64) string {
	return Number(math.Round(v), 0)
}


// Arrow returns sign arrow indicator ("▲+", "▼", " ").
func Arrow(v float64) string {
	if v > 0 {
		return "▲+"
	}
	if v < 0 {
		return "▼"
	}
	return " "
}

// ArrowNeutral returns sign arrow indicator ("▲", "▼", "─").
func ArrowNeutral(v float64) string {
	if v > 0 {
		return "▲"
	}
	if v < 0 {
		return "▼"
	}
	return "─"
}

// EokArrow formats v (in eok) into "▲+1.23조" or "N억".
func EokArrow(v float64) string {
	ar := Arrow(v)
	abs := math.Abs(v)
	if abs >= 10000 {
		return fmt.Sprintf("%s%.2f조", ar, v/10000)
	}
	return fmt.Sprintf("%s%.0f억", ar, v)
}

// AmountEok formats v (in eok) without arrows/signs.
func AmountEok(v float64) string {
	abs := math.Abs(v)
	if abs >= 10000 {
		return fmt.Sprintf("%.2f조", abs/10000)
	}
	return fmt.Sprintf("%.0f억", abs)
}

func reverse(value string) string {
	runes := []rune(value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
