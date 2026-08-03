package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fifi/internal/envcfg"
)

func env(key string) string {
	return envcfg.Get(key, "")
}

func envDefault(key string, fallback string) string {
	return envcfg.Get(key, fallback)
}

func validateDate(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "-", ""))
	if value == "" {
		return "", nil
	}
	if len(value) != 8 {
		return "", fmt.Errorf("--date must be YYYY-MM-DD or YYYYMMDD")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("--date must contain only digits after '-' removal")
		}
	}
	return value, nil
}

func validateSidecar(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unknown", "triggered", "not-triggered":
		return nil
	default:
		return fmt.Errorf("--sidecar-status must be triggered, not-triggered, or unknown")
	}
}

func parseOptionalFloat(value string, name string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("--%s must be a float: %w", name, err)
	}
	return &parsed, nil
}
