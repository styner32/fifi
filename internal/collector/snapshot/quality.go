package snapshot

import "math"

type QualityStatus string

const (
	StatusValid                          QualityStatus = "VALID"
	StatusStale                          QualityStatus = "STALE"
	StatusMissing                        QualityStatus = "MISSING"
	StatusUnavailable                    QualityStatus = "UNAVAILABLE"
	StatusInsufficientData               QualityStatus = "INSUFFICIENT_DATA"
	StatusSourceError                    QualityStatus = "SOURCE_ERROR"
	StatusParserError                    QualityStatus = "PARSER_ERROR"
	StatusTimestampMismatch              QualityStatus = "TIMESTAMP_MISMATCH"
	StatusArithmeticMismatch             QualityStatus = "ARITHMETIC_MISMATCH"
	StatusInconsistentWithRelatedMetric QualityStatus = "INCONSISTENT_WITH_RELATED_METRIC"
	StatusEstimated                      QualityStatus = "ESTIMATED"
	StatusManualInputRequired            QualityStatus = "MANUAL_INPUT_REQUIRED"
)

// ValidateCredit checking customer deposit delta.
// delta = current_balance - previous_balance. If delta differs from reported_delta by more than 2000 (2000억), return warning flags.
func ValidateCredit(current, previous, reportedDelta float64) (QualityStatus, []string) {
	if current <= 0 || previous <= 0 {
		return StatusValid, nil
	}
	diff := current - previous
	errAmt := math.Abs(diff - reportedDelta)
	if errAmt > 2000 {
		return StatusArithmeticMismatch, []string{"DEPOSIT_DELTA_MISMATCH"}
	}
	return StatusValid, nil
}

// ValidateHHI checking lower bound of HHI.
// HHI must be >= sum(weights^2) of known constituents.
func ValidateHHI(hhi float64, weights []float64) (QualityStatus, []string) {
	if hhi <= 0 {
		return StatusValid, nil
	}
	var minHHI float64
	for _, w := range weights {
		minHHI += w * w
	}
	if hhi < minHHI-0.1 { // Allow tiny rounding tolerance
		return StatusInconsistentWithRelatedMetric, []string{"HHI_LOWER_BOUND_VIOLATION"}
	}
	return StatusValid, nil
}
