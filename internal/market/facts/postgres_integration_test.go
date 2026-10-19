package facts_test

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/fifi/internal/db"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/market/facts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("PostgreSQL observation provenance", func() {
	Context("with an explicitly configured migrated test database", func() {
		It("retains legacy evidence and returns only valid current provenance with normalized dates", func() {
			dsn := os.Getenv("TEST_DATABASE_URL")
			if dsn == "" {
				Skip("TEST_DATABASE_URL is required for the isolated migration 011 integration check")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			connection, err := db.InitDBContext(ctx, dsn)
			Expect(err).NotTo(HaveOccurred())
			pool, err := connection.DB()
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(pool.Close)
			rollback := errors.New("rollback integration fixture")
			err = connection.Transaction(func(tx *gorm.DB) error {
				store := facts.NewPostgresStoreWithDB(tx)
				observed := time.Date(2026, 10, 16, 13, 0, 0, 0, kst.Location)
				value := 33.0
				current := facts.Observation{MetricID: "facts.provenance.integration", BusinessDate: "20261016", ObservedAt: observed, RetrievedAt: observed.Add(time.Minute), Value: &value, FreshnessStatus: "VALID", Source: "KIS", ProvenanceVersion: facts.CurrentProvenanceVersion, Raw: map[string]any{"original": "current"}}
				legacy := current
				legacy.ProvenanceVersion = 0
				legacy.Raw = map[string]any{"original": "legacy"}
				Expect(store.Put(ctx, legacy, current)).To(Succeed())
				future := current
				future.ObservedAt = time.Now().Add(time.Hour)
				mismatch := current
				mismatch.ObservedAt = observed.Add(time.Minute)
				mismatch.BusinessDate = "20261015"
				unknown := current
				unknown.ObservedAt = observed.Add(2 * time.Minute)
				unknown.FreshnessStatus = "TIMESTAMP_MISSING"
				// Simulate externally supplied/corrupted current rows; reads must still fail closed.
				Expect(tx.WithContext(ctx).Create(&[]facts.Observation{future, mismatch, unknown}).Error).NotTo(HaveOccurred())
				latest, err := store.LatestValid(ctx, current.MetricID, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(latest).NotTo(BeNil())
				Expect(latest.BusinessDate).To(Equal("20261016"))
				Expect(latest.ObservedAt).To(BeTemporally("==", observed))
				Expect(latest.Raw).To(HaveKeyWithValue("original", "current"))
				latest, err = store.Latest(ctx, current.MetricID)
				Expect(err).NotTo(HaveOccurred())
				Expect(latest.ProvenanceVersion).To(Equal(1))
				Expect(latest.ObservedAt).To(BeTemporally("==", observed))
				history, err := store.History(ctx, current.MetricID, time.Time{}, time.Time{}, 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(history).To(HaveLen(1))
				Expect(history[0].BusinessDate).To(Equal("20261016"))
				var preserved facts.Observation
				Expect(tx.Where("metric_id = ? AND provenance_version = 0", current.MetricID).First(&preserved).Error).NotTo(HaveOccurred())
				Expect(preserved.Raw).To(HaveKeyWithValue("original", "legacy"))
				return rollback
			})
			Expect(err).To(MatchError(rollback))
		})
	})
})
