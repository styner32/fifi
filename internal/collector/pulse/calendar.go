package pulse

import (
	"fmt"
	"time"
)

// IsHoliday checks if the given venue is closed on the specific date (YYYYMMDD).
func IsHoliday(venue string, dateStr string) bool {
	if venue == "JPX" && dateStr == "20260720" {
		return true // Marine Day in Japan
	}
	return false
}

// GetMarketPhase determines the phase of the market session.
func GetMarketPhase(venue string, t time.Time, isHoliday bool) string {
	if isHoliday {
		return "HOLIDAY"
	}

	// Weekend check
	weekday := t.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return "CLOSED"
	}

	// Normal session check
	if venue == "KRX" {
		kstTime := t.In(kstLocation)
		hm := kstTime.Hour()*100 + kstTime.Minute()
		switch {
		case hm >= 900 && hm < 1520:
			return "CONTINUOUS"
		case hm >= 1520 && hm <= 1530:
			return "CLOSING_AUCTION"
		case hm > 1530 && hm < 1800:
			return "POST_CLOSE"
		default:
			return "CLOSED"
		}
	} else if venue == "JPX" {
		jstTime := t.In(kstLocation) // KST and JST are both UTC+9
		hm := jstTime.Hour()*100 + jstTime.Minute()
		switch {
		case (hm >= 900 && hm < 1130) || (hm >= 1230 && hm < 1500):
			return "CONTINUOUS"
		default:
			return "CLOSED"
		}
	}

	return "CONTINUOUS" // Default for 24h CME futures/FX
}

// DetermineFreshness calculates the freshness category and metrics.
func DetermineFreshness(venue string, lastTS, now time.Time, isHoliday bool) (string, float64, string) {
	if isHoliday {
		return "HOLIDAY", now.Sub(lastTS).Seconds(), "Market is closed for holiday"
	}
	if lastTS.IsZero() {
		return "UNKNOWN", 0, "No timestamp available"
	}

	ageSeconds := now.Sub(lastTS).Seconds()
	if ageSeconds < 0 {
		// Clock skew fallback
		return "FRESH", ageSeconds, "Quote timestamp is in the future"
	}

	// Weekend check
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return "STALE", ageSeconds, "Weekend"
	}

	var freshLimit, delayedLimit float64
	switch venue {
	case "KRX":
		freshLimit = 120
		delayedLimit = 300
	case "USD/KRW":
		freshLimit = 120
		delayedLimit = 300
	case "CME", "NYMEX":
		freshLimit = 300
		delayedLimit = 900
	case "CBOE": // US 10Y Yield
		freshLimit = 900
		delayedLimit = 3600
	default:
		freshLimit = 300
		delayedLimit = 900
	}

	switch {
	case ageSeconds <= freshLimit:
		return "FRESH", ageSeconds, ""
	case ageSeconds <= delayedLimit:
		return "DELAYED", ageSeconds, fmt.Sprintf("Delayed by %.1fm", ageSeconds/60.0)
	default:
		return "STALE", ageSeconds, fmt.Sprintf("Stale by %.1fh", ageSeconds/3600.0)
	}
}

// CapTimeAt1530 caps the given time at 15:30 KST on the same day if it is after.
func CapTimeAt1530(t time.Time) time.Time {
	kstTime := t.In(kstLocation)
	kst330 := time.Date(kstTime.Year(), kstTime.Month(), kstTime.Day(), 15, 30, 0, 0, kstLocation)
	if kstTime.After(kst330) {
		return kst330
	}
	return t
}
