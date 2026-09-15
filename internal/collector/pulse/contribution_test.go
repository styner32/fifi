package pulse

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fifi/internal/domesticstock"
	"math"
	"strings"
	"testing"
)

func contributionFixture() contributionFake {
	s := &domesticstock.KOSPIMarketCapSummary{BusinessDate: "20260915", Universe: domesticstock.KOSPIIndexMasterUniverse, WeightStatus: domesticstock.KOSPIIndexWeightStatus}
	f := contributionFake{summary: s, changes: map[string]float64{}}
	for i := 1; i <= 11; i++ {
		code := fmt.Sprintf("%06d", i)
		s.Constituents = append(s.Constituents, domesticstock.KOSPIMarketCapConstituent{Code: code, MarketCap: float64(i), KOSPIIndexMember: true, PreferredClass: "0"})
		s.TotalMarketCap += float64(i)
		f.changes[code] = 1
	}
	return f
}
func TestContributionUsesWholeUniverseBeforeTopTenSelection(t *testing.T) {
	f := contributionFixture()
	rows, e := collectContributions(context.Background(), f, "20260915", IndexLevel{OK: true, PrevClose: 1000})
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 10 || rows[0].Code != "000011" || rows[9].Code != "000002" {
		t.Fatalf("top ten not rebuilt %+v", rows)
	}
	if rows[0].Denominator != 66 || math.Abs(rows[0].WeightPct-100.0*11/66) > .00001 || rows[0].UniverseCount != 11 {
		t.Fatalf("wrong denominator %+v", rows[0])
	}
	for _, r := range rows {
		if r.PointImpact != nil || r.Status != "NOT_EVALUATED" {
			t.Fatal("unverified point estimate published")
		}
	}
	b, _ := json.Marshal(rows)
	if !strings.Contains(string(b), `"point_impact":null`) {
		t.Fatal("missing status")
	}
	out := Render(&Pulse{Contributions: rows, Now: qualityNow()})
	if strings.Contains(out, "지수 영향합") || strings.Contains(out, "기여율 +") {
		t.Fatal("invalid aggregate still rendered")
	}
}
func TestContributionRejectsWrongUniverseDateOrDenominator(t *testing.T) {
	cases := []func(*contributionFake){func(f *contributionFake) { f.summary.TotalMarketCap += 10 }, func(f *contributionFake) { f.summary.Universe = "ALL_MARKET" }, func(f *contributionFake) { f.summary.BusinessDate = "20260914" }, func(f *contributionFake) { f.summary.Constituents[0].PreferredClass = "1" }, func(f *contributionFake) { f.summary.Constituents[0].KOSPIIndexMember = false }, func(f *contributionFake) { f.summary.Constituents[1].Code = f.summary.Constituents[0].Code }, func(f *contributionFake) { delete(f.changes, "000011") }}
	for _, mutate := range cases {
		f := contributionFixture()
		mutate(&f)
		rows, e := collectContributions(context.Background(), f, "20260915", IndexLevel{OK: true, PrevClose: 1000})
		if e == nil || len(rows) != 0 {
			t.Fatal("invalid or partial contribution published")
		}
	}
}
