package format

import (
	"testing"
)

func TestNumber(t *testing.T) {
	tests := []struct {
		val      float64
		decimals int
		expected string
	}{
		{1234567.89, 2, "1,234,567.89"},
		{-1234567.89, 2, "-1,234,567.89"},
		{1234.5678, 3, "1,234.568"},
		{123, 0, "123"},
		{0.0, 1, "0.0"},
	}

	for _, tt := range tests {
		got := Number(tt.val, tt.decimals)
		if got != tt.expected {
			t.Errorf("Number(%v, %v) = %q; expected %q", tt.val, tt.decimals, got, tt.expected)
		}
	}
}

func TestSigned(t *testing.T) {
	if got := Signed(123.4, 1); got != "+123.4" {
		t.Errorf("Signed(123.4, 1) = %q, expected \"+123.4\"", got)
	}
	if got := Signed(-123.4, 1); got != "-123.4" {
		t.Errorf("Signed(-123.4, 1) = %q, expected \"-123.4\"", got)
	}
	if got := Signed(0, 1); got != "0.0" {
		t.Errorf("Signed(0, 1) = %q, expected \"0.0\"", got)
	}
}

func TestPercent(t *testing.T) {
	if got := Percent(1.234); got != "+1.23%" {
		t.Errorf("Percent(1.234) = %q, expected \"+1.23%%\"", got)
	}
	if got := Percent(-1.234); got != "-1.23%" {
		t.Errorf("Percent(-1.234) = %q, expected \"-1.23%%\"", got)
	}
}

func TestPercentPlain(t *testing.T) {
	if got := PercentPlain(1.234); got != "1.23%" {
		t.Errorf("PercentPlain(1.234) = %q, expected \"1.23%%\"", got)
	}
}

func TestEok(t *testing.T) {
	if got := Eok(12.6); got != "+13" {
		t.Errorf("Eok(12.6) = %q, expected \"+13\"", got)
	}
	if got := Eok(-12.4); got != "-12" {
		t.Errorf("Eok(-12.4) = %q, expected \"-12\"", got)
	}
}

func TestTrillionFromEok(t *testing.T) {
	if got := TrillionFromEok(15000); got != "+1.5조원" {
		t.Errorf("TrillionFromEok(15000) = %q, expected \"+1.5조원\"", got)
	}
}

func TestArrow(t *testing.T) {
	if got := Arrow(1.5); got != "▲+" {
		t.Errorf("Arrow(1.5) = %q, expected \"▲+\"", got)
	}
	if got := Arrow(-1.5); got != "▼" {
		t.Errorf("Arrow(-1.5) = %q, expected \"▼\"", got)
	}
	if got := Arrow(0); got != " " {
		t.Errorf("Arrow(0) = %q, expected \" \"", got)
	}
}

func TestArrowNeutral(t *testing.T) {
	if got := ArrowNeutral(1.5); got != "▲" {
		t.Errorf("ArrowNeutral(1.5) = %q, expected \"▲\"", got)
	}
	if got := ArrowNeutral(-1.5); got != "▼" {
		t.Errorf("ArrowNeutral(-1.5) = %q, expected \"▼\"", got)
	}
	if got := ArrowNeutral(0); got != "─" {
		t.Errorf("ArrowNeutral(0) = %q, expected \"─\"", got)
	}
}

func TestEokArrow(t *testing.T) {
	if got := EokArrow(15000); got != "▲+1.50조" {
		t.Errorf("EokArrow(15000) = %q, expected \"▲+1.50조\"", got)
	}
	if got := EokArrow(150); got != "▲+150억" {
		t.Errorf("EokArrow(150) = %q, expected \"▲+150억\"", got)
	}
	if got := EokArrow(-15000); got != "▼-1.50조" {
		t.Errorf("EokArrow(-15000) = %q, expected \"▼-1.50조\"", got)
	}
	if got := EokArrow(-150); got != "▼-150억" {
		t.Errorf("EokArrow(-150) = %q, expected \"▼-150억\"", got)
	}
}

func TestAmountEok(t *testing.T) {
	if got := AmountEok(15000); got != "1.50조" {
		t.Errorf("AmountEok(15000) = %q, expected \"1.50조\"", got)
	}
	if got := AmountEok(-150); got != "150억" {
		t.Errorf("AmountEok(-150) = %q, expected \"150억\"", got)
	}
}
