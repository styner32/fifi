package snapshot

import (
	"context"
	"fmt"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/kst"
	"math"
	"sort"
	"strings"
	"time"
)

func sourceDate(row map[string]any) string {
	for _, k := range []string{"stck_bsop_date", "bsop_date", "business_date"} {
		if v, ok := row[k].(string); ok && validDate(v) {
			return v
		}
	}
	return ""
}
func validDate(v string) bool { _, e := time.Parse("20060102", v); return e == nil }
func sourceTime(row map[string]any) time.Time {
	d := sourceDate(row)
	if d == "" {
		return time.Time{}
	}
	for _, k := range []string{"stck_cntg_hour", "bsop_hour", "aspr_hour", "cntg_hour"} {
		if h, ok := row[k].(string); ok && len(h) == 6 {
			t, e := time.ParseInLocation("20060102150405", d+h, kst.Location)
			if e == nil {
				return t
			}
		}
	}
	return time.Time{}
}
func observationLabel(t time.Time) string {
	if t.IsZero() {
		return "TIMESTAMP_MISSING"
	}
	return t.In(kst.Location).Format("2006-01-02 15:04:05 KST")
}
func quoteTime(q yahoo.Quote) string {
	if q.MarketTimeUnix <= 0 {
		return "TIMESTAMP_MISSING"
	}
	return observationLabel(time.Unix(q.MarketTimeUnix, 0))
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func cleanQuotes(qs map[string]yahoo.Quote) map[string]yahoo.Quote {
	out := map[string]yahoo.Quote{}
	for k, q := range qs {
		if finite(q.Price) && q.Price > 0 {
			out[k] = q
		}
	}
	return out
}
func quoteChange(q yahoo.Quote) *float64 {
	if !finite(q.Price) || !finite(q.PreviousClose) || q.Price <= 0 || q.PreviousClose <= 0 {
		return nil
	}
	return ptr((q.Price/q.PreviousClose - 1) * 100)
}

// Validate the entire dated universe before calculating any subset weights.
func indexCapSummary(ctx context.Context, stock DomesticStock, date string) (*domesticstock.KOSPIMarketCapSummary, error) {
	provider, ok := stock.(interface {
		KOSPIIndexMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("index universe unavailable")
	}
	s, e := provider.KOSPIIndexMarketCapSummary(ctx, date)
	if e != nil {
		return nil, e
	}
	if s == nil || s.BusinessDate != date || s.Universe != domesticstock.KOSPIIndexMasterUniverse || s.WeightStatus != domesticstock.KOSPIIndexWeightStatus || !finite(s.TotalMarketCap) || s.TotalMarketCap <= 0 || len(s.Constituents) == 0 {
		return nil, fmt.Errorf("unverified index universe/date/denominator")
	}
	seen := map[string]bool{}
	sum := 0.0
	for _, c := range s.Constituents {
		if c.Code == "" || seen[c.Code] || !c.KOSPIIndexMember || c.PreferredClass != "0" || !finite(c.MarketCap) || c.MarketCap <= 0 {
			return nil, fmt.Errorf("invalid index constituent %s", c.Code)
		}
		seen[c.Code] = true
		sum += c.MarketCap
	}
	if math.Abs(sum-s.TotalMarketCap) > 0.01 {
		return nil, fmt.Errorf("full universe denominator mismatch")
	}
	cp := *s
	cp.Constituents = append([]domesticstock.KOSPIMarketCapConstituent(nil), s.Constituents...)
	sort.Slice(cp.Constituents, func(i, j int) bool { return cp.Constituents[i].MarketCap > cp.Constituents[j].MarketCap })
	return &cp, nil
}
func compactName(v any) string { return strings.Join(strings.Fields(fmt.Sprint(v)), "") }

func valueEokPrecise(v *float64) string {
	if v == nil {
		return "N/A"
	}
	n := *v
	if math.Abs(n) < 0.005 {
		n = 0
	}
	return signedNumber(n, 2)
}
func missingText(v string) string {
	if v == "" {
		return "미제공"
	}
	return v
}
