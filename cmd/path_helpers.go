package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
)

func resolveBusinessDateFromMarketTime(resp *auth.RESTResponse, fallback string) string {
	if strings.TrimSpace(fallback) == "" {
		fallback = time.Now().Format("20060102")
	}

	row := firstRow(resp, "output1")
	if row == nil {
		return fallback
	}

	today := normalizeYMD(fieldString(row, "today"))
	dates := []string{
		normalizeYMD(fieldString(row, "date1")),
		normalizeYMD(fieldString(row, "date2")),
		normalizeYMD(fieldString(row, "date3")),
		normalizeYMD(fieldString(row, "date4")),
		normalizeYMD(fieldString(row, "date5")),
	}

	if today != "" {
		for _, date := range dates {
			if date == today {
				return today
			}
		}

		latest := ""
		for _, date := range dates {
			if date == "" {
				continue
			}
			if date <= today && date > latest {
				latest = date
			}
		}
		if latest != "" {
			return latest
		}

		return today
	}

	latest := ""
	for _, date := range dates {
		if date > latest {
			latest = date
		}
	}
	if latest != "" {
		return latest
	}

	return fallback
}

func normalizeYMD(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 8 {
		return ""
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}

func resolveMonteCarloJSONPath(basePath string, businessDate string, symbol string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = ".cache/dcf_monte_carlo.json"
	}

	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	stem := filename
	if ext != "" {
		stem = strings.TrimSuffix(filename, ext)
	}

	parts := []string{stem}
	if normalizedDate := normalizeYMD(businessDate); normalizedDate != "" {
		parts = append(parts, normalizedDate)
	}
	symbol = strings.TrimSpace(symbol)
	if symbol != "" {
		parts = append(parts, symbol)
	}

	resolved := strings.Join(parts, ".")
	if ext == "" {
		ext = ".json"
	}
	return filepath.Join(dir, resolved+ext)
}

func resolveCompanyAnalysisJSONPath(basePath string, businessDate string, symbol string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = ".cache/company_analysis.json"
	}

	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	stem := filename
	if ext != "" {
		stem = strings.TrimSuffix(filename, ext)
	}

	parts := []string{stem}
	if normalizedDate := normalizeYMD(businessDate); normalizedDate != "" {
		parts = append(parts, normalizedDate)
	}
	symbol = strings.TrimSpace(symbol)
	if symbol != "" {
		parts = append(parts, symbol)
	}

	resolved := strings.Join(parts, ".")
	if ext == "" {
		ext = ".json"
	}
	return filepath.Join(dir, resolved+ext)
}

func resolveQuadWitchingSnapshotPath(basePath string, businessDate string, futuresCode string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = ".cache/quad_witching_snapshot.json"
	}

	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	stem := filename
	if ext != "" {
		stem = strings.TrimSuffix(filename, ext)
	}

	parts := []string{stem}
	if normalizedDate := normalizeYMD(businessDate); normalizedDate != "" {
		parts = append(parts, normalizedDate)
	}
	futuresCode = strings.TrimSpace(futuresCode)
	if futuresCode != "" {
		parts = append(parts, futuresCode)
	}

	resolved := strings.Join(parts, ".")
	if ext == "" {
		ext = ".json"
	}
	return filepath.Join(dir, resolved+ext)
}
