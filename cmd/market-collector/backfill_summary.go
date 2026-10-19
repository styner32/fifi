package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Counts describe validated submission attempts, not rows inserted: the store's
// conflict policy can keep an existing row without reporting affected rows.
type backfillSummary struct {
	Processed        int
	AcceptedAttempts int
	Skipped          int
	Errors           int
	Reasons          map[string]int
	samples          []error
}

func (s *backfillSummary) reason(reason string) {
	if s.Reasons == nil {
		s.Reasons = map[string]int{}
	}
	s.Reasons[reason]++
}
func (s *backfillSummary) skip(reason string) { s.Skipped++; s.reason(reason) }
func (s *backfillSummary) fail(reason string, err error) {
	s.Errors++
	s.reason(reason)
	if len(s.samples) < 3 {
		message := err.Error()
		if len(message) > 256 {
			message = message[:256] + "..."
		}
		s.samples = append(s.samples, fmt.Errorf("%s: %s", reason, message))
	}
}
func (s backfillSummary) Err() error {
	if s.Errors == 0 {
		return nil
	}
	return fmt.Errorf("backfill encountered %d errors (up to 3 samples): %w", s.Errors, errors.Join(s.samples...))
}
func (s backfillSummary) String() string {
	reasons := s.Reasons
	if reasons == nil {
		reasons = map[string]int{}
	}
	raw, _ := json.Marshal(reasons) // fixed reason keys; deterministic bounded summary
	return fmt.Sprintf("processed=%d accepted_attempts=%d skipped=%d errors=%d reasons=%s", s.Processed, s.AcceptedAttempts, s.Skipped, s.Errors, raw)
}

func writeBackfillSummary(w io.Writer, kind string, stats backfillSummary) {
	fmt.Fprintf(w, "[market-collector] %s backfill: %s\n", kind, stats.String())
}

func schemaSkipReason(version int) string {
	if version < 2 {
		return "legacy_schema"
	}
	return "unsupported_schema"
}
