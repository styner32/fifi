package facts_test

import (
	"context"
	"math"
	"time"

	"github.com/fifi/internal/market/facts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Observation provenance", func() {
	var now time.Time
	var observation facts.Observation
	BeforeEach(func() {
		now = time.Date(2026, 10, 16, 4, 0, 0, 0, time.UTC)
		value := 0.0
		observation = facts.Observation{ProvenanceVersion: facts.CurrentProvenanceVersion, MetricID: "metric", BusinessDate: "20261016", ObservedAt: now.Add(-time.Minute), Value: &value, FreshnessStatus: "VALID"}
	})
	Context("accepted evidence", func() {
		It("preserves a real observed zero", func() { Expect(facts.AcceptedAt(observation, now)).To(BeTrue()) })
		DescribeTable("rejects unverified evidence", func(change func(*facts.Observation)) {
			change(&observation)
			Expect(facts.AcceptedAt(observation, now)).To(BeFalse())
		}, Entry("legacy version", func(o *facts.Observation) { o.ProvenanceVersion = 0 }),
			Entry("missing source time", func(o *facts.Observation) { o.ObservedAt = time.Time{} }),
			Entry("future source time", func(o *facts.Observation) { o.ObservedAt = now.Add(time.Second) }),
			Entry("future date", func(o *facts.Observation) { o.BusinessDate = "20261017" }),
			Entry("invalid date", func(o *facts.Observation) { o.BusinessDate = "20260230" }),
			Entry("date and observed time mismatch", func(o *facts.Observation) { o.BusinessDate = "20261015" }),
			Entry("missing value", func(o *facts.Observation) { o.Value = nil }),
			Entry("non-finite value", func(o *facts.Observation) { v := math.Inf(1); o.Value = &v }),
			Entry("unknown timestamp status", func(o *facts.Observation) { o.FreshnessStatus = "TIMESTAMP_MISSING" }))
	})
	Context("Resolve", func() {
		It("surfaces a canceled database connection and leaves DB-less reads explicit", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			store, err := facts.NewStoreWithContext(ctx, "host=127.0.0.1 port=1 user=test dbname=test sslmode=disable")
			Expect(err).To(HaveOccurred())
			Expect(store).To(BeNil())
			store, err = facts.NewStoreWithContext(ctx, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(store).To(BeAssignableToTypeOf(&facts.NullStore{}))
		})
		It("does not auto-promote an unversioned source or legacy fallback", func() {
			store := newMemoryStore()
			observation.ProvenanceVersion = 0
			Expect(store.Put(context.Background(), observation)).To(Succeed())
			result := facts.Resolve(context.Background(), store, "metric", 0, func(context.Context) (facts.Observation, error) { return observation, nil })
			Expect(result.Value).To(BeNil())
			Expect(result.FreshnessStatus).To(Equal("UNAVAILABLE"))
			Expect(store.items["metric"]).To(HaveLen(1))
		})
	})
})
