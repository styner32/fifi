package main

import (
	"testing"
)

func TestValidateDate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"2026-07-05", "20260705", false},
		{"20260705", "20260705", false},
		{"", "", false},
		{"2026-07-0", "", true},
		{"abcd-ef-gh", "", true},
	}

	for _, tc := range tests {
		got, err := validateDate(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateDate(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
		}
		if got != tc.expected {
			t.Errorf("validateDate(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestValidateSidecar(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"triggered", false},
		{"not-triggered", false},
		{"unknown", false},
		{"", false},
		{"invalid-status", true},
	}

	for _, tc := range tests {
		err := validateSidecar(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateSidecar(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
		}
	}
}

func TestParseOptionalFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected *float64
		wantErr  bool
	}{
		{"", nil, false},
		{"1,234.56", floatPointer(1234.56), false},
		{"invalid", nil, true},
	}

	for _, tc := range tests {
		got, err := parseOptionalFloat(tc.input, "test")
		if (err != nil) != tc.wantErr {
			t.Errorf("parseOptionalFloat(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
		}
		if !tc.wantErr && tc.expected != nil {
			if got == nil || *got != *tc.expected {
				t.Errorf("parseOptionalFloat(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		}
	}
}

func floatPointer(v float64) *float64 {
	return &v
}
