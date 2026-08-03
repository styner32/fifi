package kst

import (
	"testing"
	"time"
)

func TestLocation(t *testing.T) {
	if Location == nil {
		t.Fatalf("Location is nil")
	}

	// Verify UTC offset of Location is +9 hours (32400 seconds)
	// We use standard time since Seoul doesn't have daylight saving time.
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, Location)
	_, offset := now.Zone()
	if offset != 9*3600 {
		t.Errorf("expected offset 32400 seconds (9 hours), got %d", offset)
	}
}

func TestNow(t *testing.T) {
	now := Now()
	if now.Location() != Location {
		t.Errorf("expected Now() to have location %v, got %v", Location, now.Location())
	}
}
