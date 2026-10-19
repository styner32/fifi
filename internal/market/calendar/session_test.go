package calendar_test

import (
	"context"
	"errors"
	"time"

	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/market/calendar"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type dayProvider struct {
	day   calendar.TradingDay
	err   error
	calls int
}

func (p *dayProvider) TradingDay(context.Context, string) (calendar.TradingDay, error) {
	p.calls++
	return p.day, p.err
}

var _ = Describe("KRX session provenance", func() {
	var now time.Time
	BeforeEach(func() { now = time.Date(2026, 10, 16, 10, 0, 0, 0, kst.Location) })
	Context("calendar availability", func() {
		It("labels regular clock hours as estimated when unavailable", func() {
			s := calendar.ResolveKRXSession(context.Background(), &dayProvider{err: errors.New("offline")}, now)
			Expect(s.Phase).To(Equal("CONTINUOUS"))
			Expect(s.Status).To(Equal("ESTIMATED"))
			Expect(s.Reason).To(ContainSubstring("offline"))
		})
		It("uses a verified closed day and does not infer live hours from an open day", func() {
			p := &dayProvider{day: calendar.TradingDay{BusinessDate: "20261016", Source: "KIS", CheckedAt: now}}
			s := calendar.ResolveKRXSession(context.Background(), p, now)
			Expect(s.ClosedDay()).To(BeTrue())
			Expect(s.Status).To(Equal("VERIFIED_CLOSED"))
			p.day.Open = true
			s = calendar.ResolveKRXSession(context.Background(), p, now)
			Expect(s.Status).To(Equal("ESTIMATED"))
			Expect(s.ClosedDay()).To(BeFalse())
		})
		It("does not call KIS on a weekend, including a UTC Friday that is KST Saturday", func() {
			p := &dayProvider{}
			s := calendar.ResolveKRXSession(context.Background(), p, time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC))
			Expect(s.Phase).To(Equal("CLOSED"))
			Expect(s.Status).To(Equal("WEEKEND"))
			Expect(p.calls).To(BeZero())
		})
	})
	Context("quote timing", func() {
		It("never marks future or unknown timestamps fresh, including holidays", func() {
			state, _, _ := calendar.DetermineFreshness("KRX", now.Add(time.Second), now, true)
			Expect(state).To(Equal("UNKNOWN"))
			state, _, _ = calendar.DetermineFreshness("KRX", time.Time{}, now, true)
			Expect(state).To(Equal("UNKNOWN"))
		})
	})
})
