package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/fifi/internal/kst"
)

type TradingDay struct {
	BusinessDate string    `json:"business_date"`
	Open         bool      `json:"open"`
	Source       string    `json:"source"`
	CheckedAt    time.Time `json:"checked_at"`
}

type TradingDayProvider interface {
	TradingDay(context.Context, string) (TradingDay, error)
}

// Session distinguishes a confirmed non-trading day from estimated clock hours.
// An open-day API response does not certify special hours or a current halt.
type Session struct {
	BusinessDate string    `json:"business_date"`
	Phase        string    `json:"phase"`
	Status       string    `json:"status"`
	Source       string    `json:"source"`
	CheckedAt    time.Time `json:"checked_at"`
	Reason       string    `json:"reason,omitempty"`
}

func ResolveKRXSession(ctx context.Context, provider TradingDayProvider, now time.Time) Session {
	local := now.In(kst.Location)
	s := Session{BusinessDate: local.Format("20060102"), Phase: GetMarketPhase("KRX", local, false), Status: "ESTIMATED", Source: "WEEKDAY_REGULAR_HOURS", CheckedAt: now,
		Reason: "regular hours estimate; special sessions and official halt history unverified"}
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		s.Phase, s.Status, s.Source, s.Reason = "CLOSED", "WEEKEND", "WEEKEND_CALENDAR", "weekend; no regular KRX session"
		return s
	}
	if provider == nil {
		s.Reason += "; trading-day provider unavailable"
		return s
	}
	day, err := provider.TradingDay(ctx, s.BusinessDate)
	if err != nil {
		s.Reason += "; " + err.Error()
		return s
	}
	if day.BusinessDate != s.BusinessDate || day.Source == "" || day.CheckedAt.IsZero() || day.CheckedAt.After(now.Add(5*time.Minute)) {
		s.Reason += "; invalid trading-day response"
		return s
	}
	s.Source, s.CheckedAt = day.Source, day.CheckedAt
	if !day.Open {
		s.Phase, s.Status, s.Reason = "HOLIDAY", "VERIFIED_CLOSED", "KIS opnd_yn=N"
	} else {
		s.Reason = "KIS open day confirmed; regular hours estimated, special sessions and halts unverified"
	}
	return s
}

func (s Session) Label() string {
	return fmt.Sprintf("%s [%s] · %s", s.Phase, s.Status, s.Source)
}

func (s Session) ClosedDay() bool {
	return s.Status == "VERIFIED_CLOSED" || s.Status == "WEEKEND"
}
