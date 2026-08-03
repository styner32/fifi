package parse

import (
	"encoding/json"
	"testing"
)

func TestFloat(t *testing.T) {
	tests := []struct {
		input    any
		expected float64
		ok       bool
	}{
		{12.34, 12.34, true},
		{float32(5.5), 5.5, true},
		{100, 100.0, true},
		{int64(200), 200.0, true},
		{int32(300), 300.0, true},
		{json.Number("123.45"), 123.45, true},
		{json.Number("bad"), 0, false},
		{"1,234.56", 1234.56, true},
		{"   -1,234.56   ", -1234.56, true},
		{"", 0, false},
		{"   ", 0, false},
		{"-", 0, false},
		{"abc", 0, false},
		{nil, 0, false},
		{struct{}{}, 0, false},
	}

	for _, tt := range tests {
		got, ok := Float(tt.input)
		if ok != tt.ok || (ok && got != tt.expected) {
			t.Errorf("Float(%v) = (%v, %v); expected (%v, %v)", tt.input, got, ok, tt.expected, tt.ok)
		}
	}
}

func TestOptionalFloat(t *testing.T) {
	if got := OptionalFloat("1,234"); got == nil || *got != 1234.0 {
		t.Errorf("OptionalFloat(\"1,234\") = %v, expected 1234.0", got)
	}
	if got := OptionalFloat("bad"); got != nil {
		t.Errorf("OptionalFloat(\"bad\") = %v, expected nil", got)
	}
}

func TestString(t *testing.T) {
	if got := String("hello"); got != "hello" {
		t.Errorf("String(\"hello\") = %q, expected \"hello\"", got)
	}
	if got := String(123); got != "123" {
		t.Errorf("String(123) = %q, expected \"123\"", got)
	}
}

func TestNum(t *testing.T) {
	row := map[string]any{
		"a": "1,200",
		"b": 45.67,
	}
	if got, ok := Num(row, "c", "b"); !ok || got != 45.67 {
		t.Errorf("Num(row, \"c\", \"b\") = (%v, %v), expected (45.67, true)", got, ok)
	}
	if got, ok := Num(row, "c", "d"); ok || got != 0 {
		t.Errorf("Num(row, \"c\", \"d\") = (%v, %v), expected (0, false)", got, ok)
	}
}

func TestInt(t *testing.T) {
	row := map[string]any{
		"a": "12.6",
		"b": 45.2,
	}
	if got, ok := Int(row, "a"); !ok || got != 13 {
		t.Errorf("Int(row, \"a\") = (%v, %v), expected (13, true)", got, ok)
	}
	if got, ok := Int(row, "b"); !ok || got != 45 {
		t.Errorf("Int(row, \"b\") = (%v, %v), expected (45, true)", got, ok)
	}
	if got, ok := Int(row, "c"); ok || got != 0 {
		t.Errorf("Int(row, \"c\") = (%v, %v), expected (0, false)", got, ok)
	}
}
