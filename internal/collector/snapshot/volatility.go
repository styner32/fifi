package snapshot

import (
	"context"
	"github.com/fifi/internal/kst"
	"sort"
	"strings"
	"time"

	"github.com/fifi/internal/market/facts"
)

// VolatilitySection은 VKOSPI/VIX 변동성 지표를 담습니다.
type VolatilitySection struct {
	Date           string        `json:"date"`
	RetrievedAt    time.Time     `json:"retrieved_at"`
	VIXObservedAt  time.Time     `json:"vix_observed_at"`
	VIXStatus      string        `json:"vix_status"`
	VKOSPIChangeOK bool          `json:"vkospi_change_ok"`
	VIXChangeOK    bool          `json:"vix_change_ok"`
	AverageDates   []string      `json:"average_dates"`
	VKOSPI         float64       `json:"vkospi"`
	VKOSPIChange   float64       `json:"vkospi_change"`
	VKOSPI5DayAvg  float64       `json:"vkospi_5day_avg"`
	VIX            float64       `json:"vix"`
	VIXChange      float64       `json:"vix_change"`
	DecouplingFlag bool          `json:"decoupling_flag"`
	Level          string        `json:"level"`
	Reason         string        `json:"reason,omitempty"`
	Source         string        `json:"source"`
	Status         QualityStatus `json:"status,omitempty"`
	QualityFlags   []string      `json:"quality_flags,omitempty"`
	ObservedAt     time.Time     `json:"observed_at,omitempty"`
}

// collectVolatility는 VKOSPI(KIS -> Naver -> facts.Store fallback)와 VIX(Yahoo Finance)를 조회합니다.
func collectVolatility(ctx context.Context, stock DomesticStock, naverClient NaverFinance, yclient YahooQuotes, store facts.Store, indexChange float64, date string, opts Options) *VolatilitySection {
	now := opts.AsOf
	if now.IsZero() {
		now = time.Now().In(kst.Location)
	}
	s := &VolatilitySection{Status: StatusUnavailable, RetrievedAt: now, VIXStatus: "UNAVAILABLE"}
	if yclient != nil {
		if qs, e := yclient.GetQuotes(ctx, []string{"^VIX"}); e == nil {
			if q, ok := cleanQuotes(qs)["^VIX"]; ok {
				s.VIX = q.Price
				s.VIXStatus = "TIMESTAMP_MISSING"
				if q.MarketTimeUnix > 0 {
					s.VIXObservedAt = time.Unix(q.MarketTimeUnix, 0)
					s.VIXStatus = "RAW_OBSERVATION"
				}
				if ch := quoteChange(q); ch != nil {
					s.VIXChange = *ch
					s.VIXChangeOK = true
				}
			}
		}
	}
	dated := map[string]float64{}
	conflict := map[string]bool{}
	if stock != nil {
		if code, e := stock.ResolveVKOSPICode(ctx, nil); e == nil {
			if rs, e := stock.InquireVKOSPIDailyPrice(ctx, code, date); e == nil {
				for _, r := range rs {
					d := sourceDate(r)
					v, ok := num(r, "bstp_nmix_prpr", "stck_clpr")
					if d != "" && d <= date && ok && v > 0 {
						if _, exists := dated[d]; exists {
							conflict[d] = true
						}
						dated[d] = v
					}
				}
			}
			if date == now.Format("20060102") {
				if resp, e := stock.InquireVKOSPIPrice(ctx, code); e == nil && resp.IsOK() {
					r := firstRow(resp, "output")
					if v, ok := num(r, "bstp_nmix_prpr"); ok && v > 0 {
						s.VKOSPI = v
						s.Source = "KIS"
						s.ObservedAt = sourceTime(r)
						s.Date = sourceDate(r)
						s.Status = StatusTimestampMissing
						if !s.ObservedAt.IsZero() {
							s.Status = StatusPreliminary
						}
						if ch, ok := num(r, "bstp_nmix_prdy_ctrt"); ok {
							s.VKOSPIChange = ch
							s.VKOSPIChangeOK = true
						}
					}
				}
			}
		}
	}
	// An explicitly dated close is preferred to an undated current quote.
	if v, ok := dated[date]; ok && !conflict[date] {
		s.VKOSPI = v
		s.Date = date
		s.Source = "KIS_DAILY"
		s.Status = StatusPreliminary
		s.ObservedAt = time.Time{}
	}
	if s.VKOSPI == 0 && naverClient != nil {
		if hist, e := naverClient.GetIndexDailyHistory(ctx, "VKOSPI", 10); e == nil {
			for _, r := range hist {
				d := strings.ReplaceAll(r.Date, ".", "")
				if d == date && finite(r.Close) && r.Close > 0 {
					s.VKOSPI = r.Close
					s.Date = d
					s.Source = "NAVER_DAILY"
					s.Status = StatusPreliminary
				}
			}
		}
	}
	// Stored fallbacks retain their original date/time and never become current data.
	if s.VKOSPI == 0 && store != nil {
		if last, e := store.LatestValid(ctx, "vkospi.value", 7*24*time.Hour); e == nil && last != nil && last.Value != nil && finite(*last.Value) && *last.Value > 0 && last.BusinessDate <= date && !last.ObservedAt.IsZero() && !last.ObservedAt.After(now) {
			s.VKOSPI = *last.Value
			s.Date = last.BusinessDate
			s.ObservedAt = last.ObservedAt
			s.Source = last.Source
			s.Status = StatusProvisionalLastValid
			s.Reason = "last valid context only"
		}
	}
	var ds []string
	for d := range dated {
		if !conflict[d] {
			ds = append(ds, d)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ds)))
	// Before the close, today's daily bar is not a completed daily observation.
	completed := []string{}
	for _, d := range ds {
		if d == date && date == now.Format("20060102") && now.Hour()*60+now.Minute() < 930 {
			continue
		}
		completed = append(completed, d)
	}
	if len(completed) >= 5 {
		sum := 0.0
		for _, d := range completed[:5] {
			sum += dated[d]
		}
		s.VKOSPI5DayAvg = sum / 5
		s.AverageDates = completed[:5]
	}
	if s.Source == "KIS_DAILY" {
		for _, d := range ds {
			if d < date {
				s.VKOSPIChange = (s.VKOSPI/dated[d] - 1) * 100
				s.VKOSPIChangeOK = true
				break
			}
		}
	}
	if s.VKOSPI > 0 {
		s.Level = vkospiLevel(s.VKOSPI)
	} else {
		s.Reason = "valid VKOSPI observation unavailable"
	}
	return s
}

func vkospiLevel(v float64) string {
	switch {
	case v < 20:
		return "정상"
	case v < 25:
		return "평상시"
	case v < 30:
		return "주의"
	default:
		return "위험"
	}
}

// isDecoupling: 지수와 VKOSPI 방향이 다르고 VKOSPI 변동폭 > 5%
func isDecoupling(indexChg, vkospiChg float64) bool {
	if vkospiChg < -5 || vkospiChg > 5 {
		return (indexChg > 0 && vkospiChg < -5) || (indexChg < 0 && vkospiChg > 5)
	}
	return false
}
