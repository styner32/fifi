package pulse

import (
	"fmt"
	"math"
	"time"

	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/parse"
)

const millionToEok = 100.0 // 백만원 → 억원

var kstLocation = kst.Location

func flowDirection(v float64) string {
	if v > 0 {
		return "순매수"
	}
	if v < 0 {
		return "순매도"
	}
	return "중립"
}

func elapsedLabel(minutes float64) string {
	if minutes <= 0 {
		return "구간"
	}
	return fmt.Sprintf("%.0fm", minutes)
}

func hourlyRate(value, elapsedMinutes float64) float64 {
	if elapsedMinutes <= 0 {
		return 0
	}
	return value * 60 / elapsedMinutes
}

func ptr(v float64) *float64 { return &v }

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validFields(row map[string]any, keys ...string) bool {
	for _, k := range keys {
		v, ok := parse.Num(row, k)
		if !ok || !finite(v) {
			return false
		}
	}
	return true
}
func stringField(row map[string]any, key string) string { s, _ := row[key].(string); return s }

// A provider clock without its observation date cannot establish freshness.
func sourceTimestamp(row map[string]any) time.Time {
	for _, d := range []string{"stck_bsop_date", "bsop_date"} {
		for _, h := range []string{"stck_cntg_hour", "bsop_hour", "aspr_hour"} {
			ds, hs := stringField(row, d), stringField(row, h)
			if len(ds) != 8 || len(hs) != 6 {
				continue
			}
			if t, e := time.ParseInLocation("20060102150405", ds+hs, kstLocation); e == nil {
				return t
			}
		}
	}
	return time.Time{}
}
func usableFreshness(s string) bool { return s == "FRESH" || s == "DELAYED" }
func sameDay(a, b time.Time) bool {
	return !a.IsZero() && !b.IsZero() && a.In(kstLocation).Format("20060102") == b.In(kstLocation).Format("20060102")
}
func validRecordTime(r *PulseRecord, now time.Time) bool {
	return r != nil && r.TS.Before(now) && sameDay(r.TS, now) && r.BusinessDate == now.In(kstLocation).Format("20060102")
}

func boundedProgramDelta(d *ProgramTradeDelta, minutes float64) *ProgramTradeDelta {
	if d == nil || d.Elapsed < minutes || d.Elapsed > minutes+5 {
		return nil
	}
	return d
}
