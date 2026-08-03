package snapshot

import (
	"fmt"
	"math"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/parse"
)

func rows(resp *auth.RESTResponse, key string) []map[string]any {
	return resp.Rows(key)
}

func firstRow(resp *auth.RESTResponse, keys ...string) map[string]any {
	return resp.FirstRow(keys...)
}

func num(row map[string]any, keys ...string) (float64, bool) {
	return parse.Num(row, keys...)
}

func parseNumber(value any) (float64, bool) {
	return parse.Float(value)
}

func ptr(v float64) *float64 { return &v }

func absPercent(numerator float64, denominator float64) *float64 {
	if denominator == 0 || math.IsNaN(denominator) || math.IsInf(denominator, 0) {
		return nil
	}
	return ptr(math.Abs(numerator) / math.Abs(denominator) * 100)
}

func signedPercent(numerator float64, denominator float64) *float64 {
	if denominator == 0 || math.IsNaN(denominator) || math.IsInf(denominator, 0) {
		return nil
	}
	return ptr(numerator / denominator * 100)
}


func errText(err error) string {
	if err == nil {
		return "manual input not provided"
	}
	return fmt.Sprintf("%v", err)
}
