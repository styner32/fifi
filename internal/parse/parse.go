package parse

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Float parses various types (float64, float32, int, int64, int32, json.Number, string with commas) into a float64.
func Float(v any) (float64, bool) {
	switch typed := v.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		clean := strings.ReplaceAll(strings.TrimSpace(typed), ",", "")
		if clean == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(clean, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

// OptionalFloat is a helper wrapper around Float returning a pointer to float64 or nil.
func OptionalFloat(v any) *float64 {
	parsed, ok := Float(v)
	if !ok {
		return nil
	}
	return &parsed
}

// String converts various types to string.
func String(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Num parses the first found float64 from the map row using the provided keys.
func Num(row map[string]any, keys ...string) (float64, bool) {
	if row == nil {
		return 0, false
	}
	for _, key := range keys {
		if v, ok := Float(row[key]); ok {
			return v, true
		}
	}
	return 0, false
}

// Int parses the first found float64 from the map row using the keys, rounds it, and returns it as an int.
func Int(row map[string]any, keys ...string) (int, bool) {
	if row == nil {
		return 0, false
	}
	for _, key := range keys {
		if v, ok := Float(row[key]); ok {
			return int(math.Round(v)), true
		}
	}
	return 0, false
}
