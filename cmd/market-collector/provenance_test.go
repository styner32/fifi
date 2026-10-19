package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/market/calendar"
	"github.com/fifi/internal/market/facts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type captureStore struct {
	facts.NullStore
	observations []facts.Observation
}

func (s *captureStore) Put(_ context.Context, observations ...facts.Observation) error {
	s.observations = append(s.observations, observations...)
	return nil
}

var _ = Describe("Market observation collection and backfill", func() {
	var now time.Time
	BeforeEach(func() { now = time.Date(2026, 10, 16, 13, 0, 0, 0, kst.Location) })
	Context("persistence and closed-day controls", func() {
		It("requires explicit persistent storage instead of counting no-op writes", func() {
			GinkgoT().Setenv("DATABASE_URL", "")
			store, err := newCollectorStore(context.Background())
			Expect(store).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("DATABASE_URL is required")))
		})
		DescribeTable("never bypasses a confirmed closed day with force", func(session calendar.Session, force, expected bool) {
			Expect(shouldCollect(session, force)).To(Equal(expected))
		}, Entry("verified holiday", calendar.Session{Phase: "HOLIDAY", Status: "VERIFIED_CLOSED"}, true, false),
			Entry("weekend", calendar.Session{Phase: "CLOSED", Status: "WEEKEND"}, true, false),
			Entry("regular hours", calendar.Session{Phase: "CONTINUOUS", Status: "ESTIMATED"}, false, true),
			Entry("outside regular hours", calendar.Session{Phase: "CLOSED", Status: "ESTIMATED"}, false, false),
			Entry("forced diagnostic during unconfirmed hours", calendar.Session{Phase: "CLOSED", Status: "ESTIMATED"}, true, true))
	})
	Context("source observations", func() {
		It("preserves source timestamp, raw payload, actual zero and million-KRW units", func() {
			row := map[string]any{"business_date": "20261016", "aspr_hour": "125959", "flow": "0"}
			obs, err := sourceObservation("flow", "flow", "억원", 100, row, now)
			Expect(err).NotTo(HaveOccurred())
			Expect(*obs.Value).To(BeZero())
			Expect(obs.ObservedAt).To(Equal(now.Add(-time.Second)))
			Expect(obs.ProvenanceVersion).To(Equal(1))
			Expect(obs.Raw).To(Equal(row))
			row["flow"] = "12,345"
			obs, err = sourceObservation("flow", "flow", "억원", 100, row, now)
			Expect(err).NotTo(HaveOccurred())
			Expect(*obs.Value).To(Equal(123.45))
		})
		DescribeTable("never substitutes retrieval time for an unknown observation", func(row map[string]any) {
			_, err := sourceObservation("metric", "value", "index", 1, row, now)
			Expect(err).To(HaveOccurred())
		}, Entry("missing date", map[string]any{"aspr_hour": "120000", "value": "10"}),
			Entry("missing time", map[string]any{"business_date": "20261016", "value": "10"}),
			Entry("future", map[string]any{"business_date": "20261016", "aspr_hour": "130001", "value": "10"}),
			Entry("invalid date", map[string]any{"business_date": "20260230", "aspr_hour": "120000", "value": "10"}),
			Entry("nonfinite", map[string]any{"business_date": "20261016", "aspr_hour": "120000", "value": "NaN"}))
	})
	Context("typed schema 2 backfill", func() {
		It("reads the real pulse ts and vkospi object while rejecting legacy rows", func() {
			dir := GinkgoT().TempDir()
			record := pulse.PulseRecord{SchemaVersion: 2, TS: now, BusinessDate: "20261016", VKOSPI: pulse.VolatilitySnapshot{OK: true, Value: 33, Source: "KIS", LastTS: now.Add(-time.Minute), Freshness: "FRESH"}}
			body, err := json.Marshal(record)
			Expect(err).NotTo(HaveOccurred())
			body = append(body, []byte("\n{\"schema_version\":1,\"venue\":\"KRX\",\"vkospi\":999}\n")...)
			Expect(os.WriteFile(filepath.Join(dir, "pulse_20261016.jsonl"), body, 0600)).To(Succeed())
			store := &captureStore{}
			count, err := backfillPulse(context.Background(), store, dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
			Expect(store.observations).To(HaveLen(1))
			Expect(store.observations[0].ObservedAt).To(BeTemporally("==", now.Add(-time.Minute)))
			Expect(*store.observations[0].Value).To(Equal(33.0))
			Expect(store.observations[0].Raw).To(HaveKey("ts"))
		})
		It("refuses to invent 15:30 or promote a provisional stored fallback", func() {
			rec := snapshot.SnapshotJSON{SchemaVersion: 2, Date: "20261016", Timestamp: now, Volatility: &snapshot.VolatilitySection{Date: "20261016", VKOSPI: 33, Source: "KIS_DAILY", Status: snapshot.StatusPreliminary}}
			_, ok := snapshotObservation(rec, nil, now)
			Expect(ok).To(BeFalse())
			rec.Volatility.ObservedAt = now.Add(-time.Minute)
			obs, ok := snapshotObservation(rec, nil, now)
			Expect(ok).To(BeTrue())
			Expect(obs.ObservedAt.Hour()).To(Equal(12))
			rec.Volatility.Status = snapshot.StatusProvisionalLastValid
			_, ok = snapshotObservation(rec, nil, now)
			Expect(ok).To(BeFalse())
		})
		It("uses snapshot source date and timestamp and ignores preserved revisions", func() {
			dir := GinkgoT().TempDir()
			rec := snapshot.SnapshotJSON{SchemaVersion: 2, Date: "20261016", Timestamp: now, Volatility: &snapshot.VolatilitySection{Date: "20261016", VKOSPI: 33, Source: "KIS", ObservedAt: now.Add(-time.Minute), Status: snapshot.StatusPreliminary}}
			body, err := json.Marshal(rec)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(dir, "market_snapshot.20261016.json"), body, 0600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "market_snapshot.20261016.previous.1.json"), body, 0600)).To(Succeed())
			store := &captureStore{}
			count, err := backfillSnapshots(context.Background(), store, dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
			Expect(store.observations[0].BusinessDate).To(Equal("20261016"))
		})
	})
})
