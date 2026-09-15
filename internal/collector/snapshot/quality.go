package snapshot

import "math"

type QualityStatus string

const (
	StatusValid                         QualityStatus = "VALID"
	StatusStale                         QualityStatus = "STALE"
	StatusMissing                       QualityStatus = "MISSING"
	StatusUnavailable                   QualityStatus = "UNAVAILABLE"
	StatusInsufficientData              QualityStatus = "INSUFFICIENT_DATA"
	StatusSourceError                   QualityStatus = "SOURCE_ERROR"
	StatusParserError                   QualityStatus = "PARSER_ERROR"
	StatusTimestampMismatch             QualityStatus = "TIMESTAMP_MISMATCH"
	StatusArithmeticMismatch            QualityStatus = "ARITHMETIC_MISMATCH"
	StatusInconsistentWithRelatedMetric QualityStatus = "INCONSISTENT_WITH_RELATED_METRIC"
	StatusEstimated                     QualityStatus = "ESTIMATED"
	StatusManualInputRequired           QualityStatus = "MANUAL_INPUT_REQUIRED"
)

// ValidateCredit checking customer deposit delta.
// delta = current_balance - previous_balance. If delta differs from reported_delta by more than 1 eok (source rounding tolerance), return warning flags.
func ValidateCredit(current, previous, reportedDelta float64) (QualityStatus, []string) {
	if !finite(current) || !finite(previous) || !finite(reportedDelta) || current <= 0 || previous <= 0 {
		return StatusInsufficientData, []string{"CREDIT_INPUT_MISSING"}
	}
	diff := current - previous
	errAmt := math.Abs(diff - reportedDelta)
	if errAmt > 1 {
		return StatusArithmeticMismatch, []string{"DEPOSIT_DELTA_MISMATCH"}
	}
	return StatusValid, nil
}

// ValidateHHI checking lower bound of HHI.
// HHI must be >= sum(weights^2) of known constituents.
func ValidateHHI(hhi float64, weights []float64) (QualityStatus, []string) {
	if !finite(hhi) || hhi <= 0 {
		return StatusInsufficientData, []string{"HHI_INPUT_MISSING"}
	}
	var minHHI float64
	for _, w := range weights {
		if !finite(w) || w < 0 || w > 100 {
			return StatusInsufficientData, []string{"INVALID_WEIGHT"}
		}
		minHHI += w * w
	}
	if hhi < minHHI-0.1 { // Allow tiny rounding tolerance
		return StatusInconsistentWithRelatedMetric, []string{"HHI_LOWER_BOUND_VIOLATION"}
	}
	return StatusValid, nil
}
