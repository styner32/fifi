package calendar_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/market/calendar"
)

func TestCalendar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Market Calendar Suite")
}

var _ = Describe("Market Calendar", func() {
	Context("GetMarketPhase", func() {
		It("recognizes KRX sessions accurately", func() {
			// Monday 10:30 KST -> CONTINUOUS
			mon1030 := time.Date(2026, 8, 24, 10, 30, 0, 0, kst.Location)
			Expect(calendar.GetMarketPhase("KRX", mon1030, false)).To(Equal("CONTINUOUS"))

			// Monday 15:25 KST -> CLOSING_AUCTION
			mon1525 := time.Date(2026, 8, 24, 15, 25, 0, 0, kst.Location)
			Expect(calendar.GetMarketPhase("KRX", mon1525, false)).To(Equal("CLOSING_AUCTION"))

			// Monday 16:00 KST -> POST_CLOSE
			mon1600 := time.Date(2026, 8, 24, 16, 0, 0, 0, kst.Location)
			Expect(calendar.GetMarketPhase("KRX", mon1600, false)).To(Equal("POST_CLOSE"))

			// Monday 20:00 KST -> CLOSED
			mon2000 := time.Date(2026, 8, 24, 20, 0, 0, 0, kst.Location)
			Expect(calendar.GetMarketPhase("KRX", mon2000, false)).To(Equal("CLOSED"))

			// Saturday 10:00 KST -> CLOSED
			sat1000 := time.Date(2026, 8, 22, 10, 0, 0, 0, kst.Location)
			Expect(calendar.GetMarketPhase("KRX", sat1000, false)).To(Equal("CLOSED"))

			// Holiday -> HOLIDAY
			Expect(calendar.GetMarketPhase("KRX", mon1030, true)).To(Equal("HOLIDAY"))
		})
	})

	Context("DetermineFreshness", func() {
		It("categorizes freshness correctly", func() {
			now := time.Date(2026, 8, 24, 11, 0, 0, 0, kst.Location)
			lastFresh := now.Add(-60 * time.Second)
			status, _, _ := calendar.DetermineFreshness("KRX", lastFresh, now, false)
			Expect(status).To(Equal("FRESH"))

			lastDelayed := now.Add(-180 * time.Second)
			status, _, _ = calendar.DetermineFreshness("KRX", lastDelayed, now, false)
			Expect(status).To(Equal("DELAYED"))

			lastStale := now.Add(-600 * time.Second)
			status, _, _ = calendar.DetermineFreshness("KRX", lastStale, now, false)
			Expect(status).To(Equal("STALE"))
		})
	})

	Context("CapTimeAt1530", func() {
		It("caps time correctly", func() {
			t1600 := time.Date(2026, 8, 24, 16, 0, 0, 0, kst.Location)
			capped := calendar.CapTimeAt1530(t1600)
			Expect(capped.Hour()).To(Equal(15))
			Expect(capped.Minute()).To(Equal(30))

			t1100 := time.Date(2026, 8, 24, 11, 0, 0, 0, kst.Location)
			Expect(calendar.CapTimeAt1530(t1100)).To(Equal(t1100))
		})
	})
})
