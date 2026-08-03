package main

import (
	"testing"

	"github.com/fifi/internal/auth"
)

func TestResolveBusinessDateFromMarketTime(t *testing.T) {
	resp := &auth.RESTResponse{
		Body: map[string]any{
			"output1": map[string]any{
				"today": "20260317",
				"date1": "20260317",
				"date2": "20260318",
				"date3": "20260319",
			},
		},
	}

	if got := resolveBusinessDateFromMarketTime(resp, "20260316"); got != "20260317" {
		t.Fatalf("resolveBusinessDateFromMarketTime() = %s, want 20260317", got)
	}
}

func TestResolveBusinessDateFromMarketTimeFallsBackToLatestBusinessDate(t *testing.T) {
	resp := &auth.RESTResponse{
		Body: map[string]any{
			"output1": map[string]any{
				"today": "20260317",
				"date1": "20260314",
				"date2": "20260316",
				"date3": "20260318",
			},
		},
	}

	if got := resolveBusinessDateFromMarketTime(resp, "20260315"); got != "20260316" {
		t.Fatalf("resolveBusinessDateFromMarketTime() = %s, want 20260316", got)
	}
}

func TestDatedJSONPathResolvers(t *testing.T) {
	if got := resolveMonteCarloJSONPath(".cache/dcf_monte_carlo.json", "20260317", "005930"); got != ".cache/dcf_monte_carlo.20260317.005930.json" {
		t.Fatalf("resolveMonteCarloJSONPath() = %s", got)
	}
	if got := resolveCompanyAnalysisJSONPath(".cache/company_analysis.json", "20260317", "NVDA"); got != ".cache/company_analysis.20260317.NVDA.json" {
		t.Fatalf("resolveCompanyAnalysisJSONPath() = %s", got)
	}
	if got := resolveQuadWitchingSnapshotPath(".cache/quad_witching_snapshot.json", "20260317", "A01603"); got != ".cache/quad_witching_snapshot.20260317.A01603.json" {
		t.Fatalf("resolveQuadWitchingSnapshotPath() = %s", got)
	}
}

func TestEnvDefaultHelpers(t *testing.T) {
	t.Setenv("CMD_TEST_STRING", "value")
	t.Setenv("CMD_TEST_FLOAT", "1.25")
	t.Setenv("CMD_TEST_INT", "7")
	t.Setenv("CMD_TEST_BOOL", "true")
	t.Setenv("CMD_TEST_OPTIONAL_FLOAT", "0.75")

	if got := getOrDefault("CMD_TEST_STRING", "fallback"); got != "value" {
		t.Fatalf("getOrDefault() = %s", got)
	}
	if got := getOrDefault("CMD_TEST_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("getOrDefault() default = %s", got)
	}
	if got := getFloatOrDefault("CMD_TEST_FLOAT", 0.5); got != 1.25 {
		t.Fatalf("getFloatOrDefault() = %f", got)
	}
	if got := getIntOrDefault("CMD_TEST_INT", 3); got != 7 {
		t.Fatalf("getIntOrDefault() = %d", got)
	}
	if got := getBoolOrDefault("CMD_TEST_BOOL", false); !got {
		t.Fatalf("getBoolOrDefault() = false, want true")
	}
	if got := getOptionalFloat("CMD_TEST_OPTIONAL_FLOAT"); got == nil || *got != 0.75 {
		t.Fatalf("getOptionalFloat() = %v", got)
	}
	if got := getOptionalFloat("CMD_TEST_OPTIONAL_FLOAT_MISSING"); got != nil {
		t.Fatalf("getOptionalFloat() = %v, want nil", got)
	}
}
