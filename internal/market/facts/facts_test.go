package facts_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/market/facts"
)

func TestFacts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Facts Suite")
}

type memoryStore struct {
	items map[string][]facts.Observation
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: make(map[string][]facts.Observation)}
}

func (m *memoryStore) Put(ctx context.Context, obs ...facts.Observation) error {
	for _, o := range obs {
		m.items[o.MetricID] = append(m.items[o.MetricID], o)
	}
	return nil
}

func (m *memoryStore) Latest(ctx context.Context, metricID string) (*facts.Observation, error) {
	list := m.items[metricID]
	if len(list) == 0 {
		return nil, nil
	}
	return &list[len(list)-1], nil
}

func (m *memoryStore) LatestValid(ctx context.Context, metricID string, maxAge time.Duration) (*facts.Observation, error) {
	list := m.items[metricID]
	for i := len(list) - 1; i >= 0; i-- {
		item := list[i]
		if item.FreshnessStatus == "VALID" || item.FreshnessStatus == "FRESH" || item.FreshnessStatus == "PROVISIONAL_LAST_VALID" {
			if maxAge <= 0 || time.Since(item.ObservedAt) <= maxAge {
				return &item, nil
			}
		}
	}
	return nil, nil
}

func (m *memoryStore) History(ctx context.Context, metricID string, from, to time.Time, limit int) ([]facts.Observation, error) {
	return m.items[metricID], nil
}

var _ = Describe("Facts Observation Store & Resolve", func() {
	Context("NullStore", func() {
		It("performs no-op safely", func() {
			store := facts.NewNullStore()
			ctx := context.Background()
			val := 50.0
			obs := facts.Observation{MetricID: "test.val", Value: &val, FreshnessStatus: "VALID"}

			Expect(store.Put(ctx, obs)).To(Succeed())

			latest, err := store.Latest(ctx, "test.val")
			Expect(err).NotTo(HaveOccurred())
			Expect(latest).To(BeNil())

			latestValid, err := store.LatestValid(ctx, "test.val", 24*time.Hour)
			Expect(err).NotTo(HaveOccurred())
			Expect(latestValid).To(BeNil())

			history, err := store.History(ctx, "test.val", time.Time{}, time.Time{}, 10)
			Expect(err).NotTo(HaveOccurred())
			Expect(history).To(BeNil())
		})
	})

	Context("Resolve", func() {
		It("returns VALID when fetch succeeds and persists observation", func() {
			store := newMemoryStore()
			ctx := context.Background()
			val := 28.5

			obs := facts.Resolve(ctx, store, "vkospi.value", 7*24*time.Hour, func(c context.Context) (facts.Observation, error) {
				return facts.Observation{
					MetricID:     "vkospi.value",
					Value:        &val,
					Source:       "KIS",
					BusinessDate: "20260821",
					ObservedAt:   time.Now(),
				}, nil
			})

			Expect(obs.FreshnessStatus).To(Equal("VALID"))
			Expect(*obs.Value).To(Equal(28.5))
			Expect(obs.Source).To(Equal("KIS"))

			latest, err := store.Latest(ctx, "vkospi.value")
			Expect(err).NotTo(HaveOccurred())
			Expect(latest).NotTo(BeNil())
			Expect(*latest.Value).To(Equal(28.5))
		})

		It("falls back to PROVISIONAL_LAST_VALID when fetch fails but store has recent valid observation", func() {
			store := newMemoryStore()
			ctx := context.Background()
			prevVal := 27.8
			yesterday := time.Now().Add(-24 * time.Hour)

			_ = store.Put(ctx, facts.Observation{
				MetricID:        "vkospi.value",
				Value:           &prevVal,
				Source:          "KIS",
				BusinessDate:    "20260820",
				ObservedAt:      yesterday,
				FreshnessStatus: "VALID",
			})

			obs := facts.Resolve(ctx, store, "vkospi.value", 7*24*time.Hour, func(c context.Context) (facts.Observation, error) {
				return facts.Observation{}, errors.New("KIS timeout")
			})

			Expect(obs.FreshnessStatus).To(Equal("PROVISIONAL_LAST_VALID"))
			Expect(*obs.Value).To(Equal(27.8))
			Expect(obs.Source).To(Equal("KIS"))
			Expect(obs.ObservedAt).To(Equal(yesterday))
			Expect(obs.MissingReason).To(ContainSubstring("20260820"))
		})

		It("returns UNAVAILABLE when fetch fails and store has no recent observations", func() {
			store := newMemoryStore()
			ctx := context.Background()

			obs := facts.Resolve(ctx, store, "vkospi.value", 7*24*time.Hour, func(c context.Context) (facts.Observation, error) {
				return facts.Observation{}, errors.New("network unreachable")
			})

			Expect(obs.FreshnessStatus).To(Equal("UNAVAILABLE"))
			Expect(obs.Value).To(BeNil())
			Expect(obs.MissingReason).To(ContainSubstring("network unreachable"))
		})
	})
})
