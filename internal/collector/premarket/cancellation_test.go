package premarket_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/kst"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Premarket cancellation persistence", func() {
	Context("with an existing accepted report for the same date", func() {
		It("keeps stored evidence byte-for-byte when the actual collector is canceled", func() {
			dir := GinkgoT().TempDir()
			now := time.Date(2026, 10, 15, 8, 30, 0, 0, kst.Location)
			value := 33.0
			store := premarket.NewStore(dir)
			store.UpsertRecord(premarket.DailyRecord{Date: "20261015", ObservedAt: now.Add(-time.Minute), Inputs: map[string]premarket.Observation{"vkospi": {Value: &value, Status: "VALID", Source: "KIS_DAILY", BusinessDate: "20261014", Session: "DAILY_CLOSE"}}})
			Expect(store.Save()).To(Succeed())
			path := filepath.Join(dir, "store.v2.json")
			before, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			report := premarket.Collect(ctx, premarket.Deps{Clock: func() time.Time { return now }}, premarket.Options{StoreDir: dir})
			Expect(report.Errors).To(HaveKeyWithValue("context", context.Canceled.Error()))
			after, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(before))
		})
	})
})
