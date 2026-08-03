package pulse

import (
	"fmt"

	"github.com/fifi/internal/kst"
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

