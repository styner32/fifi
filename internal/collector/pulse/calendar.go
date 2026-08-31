package pulse

import (
	"time"

	"github.com/fifi/internal/market/calendar"
)

// IsHoliday checks if the given venue is closed on the specific date (YYYYMMDD).
func IsHoliday(venue string, dateStr string) bool {
	return calendar.IsHoliday(venue, dateStr)
}

// GetMarketPhase determines the phase of the market session.
func GetMarketPhase(venue string, t time.Time, isHoliday bool) string {
	return calendar.GetMarketPhase(venue, t, isHoliday)
}

// DetermineFreshness calculates the freshness category and metrics.
func DetermineFreshness(venue string, lastTS, now time.Time, isHoliday bool) (string, float64, string) {
	return calendar.DetermineFreshness(venue, lastTS, now, isHoliday)
}

// CapTimeAt1530 caps the given time at 15:30 KST on the same day if it is after.
func CapTimeAt1530(t time.Time) time.Time {
	return calendar.CapTimeAt1530(t)
}
