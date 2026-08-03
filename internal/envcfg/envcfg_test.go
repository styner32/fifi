package envcfg

import (
	"testing"
)

func TestGet(t *testing.T) {
	t.Setenv("TEST_GET", "value")
	if got := Get("TEST_GET", "default"); got != "value" {
		t.Errorf("Get() = %q, expected \"value\"", got)
	}
	if got := Get("TEST_GET_MISSING", "default"); got != "default" {
		t.Errorf("Get() missing = %q, expected \"default\"", got)
	}
}

func TestFloat(t *testing.T) {
	t.Setenv("TEST_FLOAT", "123.45")
	if got := Float("TEST_FLOAT", 1.0); got != 123.45 {
		t.Errorf("Float() = %v, expected 123.45", got)
	}
	if got := Float("TEST_FLOAT_MISSING", 1.0); got != 1.0 {
		t.Errorf("Float() missing = %v, expected 1.0", got)
	}
}

func TestInt(t *testing.T) {
	t.Setenv("TEST_INT", "100")
	if got := Int("TEST_INT", 1); got != 100 {
		t.Errorf("Int() = %v, expected 100", got)
	}
	if got := Int("TEST_INT_MISSING", 1); got != 1 {
		t.Errorf("Int() missing = %v, expected 1", got)
	}
}

func TestBool(t *testing.T) {
	t.Setenv("TEST_BOOL", "true")
	if got := Bool("TEST_BOOL", false); got != true {
		t.Errorf("Bool() = %v, expected true", got)
	}
	if got := Bool("TEST_BOOL_MISSING", false); got != false {
		t.Errorf("Bool() missing = %v, expected false", got)
	}
}

func TestOptionalFloat(t *testing.T) {
	t.Setenv("TEST_OPT_FLOAT", "45.67")
	if got := OptionalFloat("TEST_OPT_FLOAT"); got == nil || *got != 45.67 {
		t.Errorf("OptionalFloat() = %v, expected 45.67", got)
	}
	if got := OptionalFloat("TEST_OPT_FLOAT_MISSING"); got != nil {
		t.Errorf("OptionalFloat() missing = %v, expected nil", got)
	}
}
