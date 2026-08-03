package envcfg

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Get retrieves the environment variable key, returning fallback if empty.
func Get(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

// Float retrieves the environment variable key as float64, returning fallback if empty. Fatals if parsing fails.
func Float(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Fatalf("envcfg: %s must be a float: %v", key, err)
	}
	return parsed
}

// Int retrieves the environment variable key as int, returning fallback if empty. Fatals if parsing fails.
func Int(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("envcfg: %s must be an integer: %v", key, err)
	}
	return parsed
}

// Bool retrieves the environment variable key as bool, returning fallback if empty. Fatals if parsing fails.
func Bool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("envcfg: %s must be a boolean: %v", key, err)
	}
	return parsed
}

// OptionalFloat retrieves the environment variable key as float64 pointer, returning nil if empty. Fatals if parsing fails.
func OptionalFloat(key string) *float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Fatalf("envcfg: %s must be a float: %v", key, err)
	}
	return &parsed
}
