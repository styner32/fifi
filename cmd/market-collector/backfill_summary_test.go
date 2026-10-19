package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/kst"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Backfill accounting", func() {
	var now time.Time
	BeforeEach(func() { now = time.Date(2026, 10, 16, 13, 0, 0, 0, kst.Location) })
	Context("legacy, invalid and accepted evidence", func() {
		It("counts every pulse outcome and renders a bounded reason summary for stderr", func() {
			dir := GinkgoT().TempDir()
			valid := pulse.PulseRecord{SchemaVersion: 2, TS: now, BusinessDate: "20261016", VKOSPI: pulse.VolatilitySnapshot{OK: true, Source: "KIS", Value: 33, LastTS: now.Add(-time.Minute), Freshness: "FRESH"}}
			missingTime := valid
			missingTime.VKOSPI.LastTS = time.Time{}
			unknownStatus := valid
			unknownStatus.VKOSPI.Freshness = "UNKNOWN"
			wrongDate := valid
			wrongDate.BusinessDate = "20261015"
			var input bytes.Buffer
			encoder := json.NewEncoder(&input)
			for _, record := range []any{valid, map[string]any{"schema_version": 1, "vkospi": 999}, map[string]any{"schema_version": 3}, missingTime, unknownStatus, wrongDate} {
				Expect(encoder.Encode(record)).To(Succeed())
			}
			input.WriteString("{invalid JSON}\n")
			Expect(os.WriteFile(filepath.Join(dir, "pulse_20261016.jsonl"), input.Bytes(), 0600)).To(Succeed())
			store := &captureStore{}
			stats, err := backfillPulseWithSummary(context.Background(), store, dir)
			Expect(err).To(MatchError(ContainSubstring("1 errors")))
			Expect(stats.Processed).To(Equal(7))
			Expect(stats.AcceptedAttempts).To(Equal(1))
			Expect(stats.Skipped).To(Equal(5))
			Expect(stats.Errors).To(Equal(1))
			Expect(stats.Reasons).To(Equal(map[string]int{"legacy_schema": 1, "unsupported_schema": 1, "source_timestamp_missing": 1, "source_status_unusable": 1, "date_conflict": 1, "invalid_json": 1}))
			Expect(store.observations).To(HaveLen(1))
			Expect(store.observations[0].Unit).To(Equal("index"))
			var diagnostics bytes.Buffer
			writeBackfillSummary(&diagnostics, "pulse", stats)
			Expect(diagnostics.String()).To(ContainSubstring("processed=7 accepted_attempts=1 skipped=5 errors=1"))
			Expect(diagnostics.String()).NotTo(ContainSubstring("inserted"))
			Expect(len(diagnostics.String())).To(BeNumerically("<", 1024))
		})
		It("counts archive exclusions and refuses a snapshot whose filename conflicts with its source date", func() {
			dir := GinkgoT().TempDir()
			record := snapshot.SnapshotJSON{SchemaVersion: 2, Date: "20261016", Timestamp: now, Volatility: &snapshot.VolatilitySection{Date: "20261016", Source: "KIS", VKOSPI: 33, ObservedAt: now.Add(-time.Minute), Status: snapshot.StatusPreliminary}}
			raw, err := json.Marshal(record)
			Expect(err).NotTo(HaveOccurred())
			for _, name := range []string{"market_snapshot.20261016.json", "market_snapshot.20261016.previous.1.json", "market_snapshot.20261015.json"} {
				Expect(os.WriteFile(filepath.Join(dir, name), raw, 0600)).To(Succeed())
			}
			store := &captureStore{}
			stats, err := backfillSnapshotsWithSummary(context.Background(), store, dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(stats.Processed).To(Equal(3))
			Expect(stats.AcceptedAttempts).To(Equal(1))
			Expect(stats.Skipped).To(Equal(2))
			Expect(stats.Errors).To(BeZero())
			Expect(stats.Reasons).To(Equal(map[string]int{"archive_or_noncanonical_filename": 1, "date_conflict": 1}))
			Expect(store.observations[0].Unit).To(Equal("index"))
		})
	})
	Context("many failures", func() {
		It("counts all errors while retaining only bounded diagnostic samples", func() {
			stats := backfillSummary{}
			for i := 0; i < 1000; i++ {
				stats.fail("store_write_error", errors.New(strings.Repeat("x", 2000)))
			}
			Expect(stats.Errors).To(Equal(1000))
			Expect(stats.Reasons["store_write_error"]).To(Equal(1000))
			Expect(len(stats.Err().Error())).To(BeNumerically("<", 1024))
			Expect(len(stats.String())).To(BeNumerically("<", 256))
		})
	})
})
