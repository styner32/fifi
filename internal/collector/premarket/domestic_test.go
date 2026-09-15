package premarket

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/external/dataapi"
	"math"
	"strings"
	"testing"
	"time"
)

type extensionStock struct {
	regressionStock
	dates     []string
	gap       string
	flowCalls int
}

func newExtensionStock() *extensionStock {
	s := &extensionStock{}
	d := time.Date(2026, 9, 11, 0, 0, 0, 0, kst)
	for len(s.dates) < 21 {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			s.dates = append(s.dates, d.Format("20060102"))
		}
		d = d.AddDate(0, 0, -1)
	}
	return s
}
func (s *extensionStock) InquireIndexDailyPrice(_ context.Context, code, date string) ([]map[string]any, error) {
	s.called(code)
	out := []map[string]any{}
	for i, d := range s.dates {
		turnover := "10000"
		if i == 0 {
			turnover = "20000"
		}
		out = append(out, map[string]any{"stck_bsop_date": d, "bstp_nmix_prpr": "100", "bstp_nmix_prdy_ctrt": "0", "acml_tr_pbmn": turnover})
	}
	out = append(out, map[string]any{"stck_bsop_date": "20260914", "bstp_nmix_prpr": "999", "acml_tr_pbmn": "999999"})
	return out, nil
}
func (s *extensionStock) flowRows() []any {
	out := []any{}
	for i, d := range s.dates {
		if d == s.gap {
			continue
		}
		out = append(out, map[string]any{"stck_bsop_date": d, "frgn_ntby_tr_pbmn": fmt.Sprint(i * 100), "orgn_ntby_tr_pbmn": "-200", "prsn_ntby_tr_pbmn": "100"})
	}
	return out
}
func (s *extensionStock) InquireInvestorDailyByMarket(context.Context, string) (*auth.RESTResponse, error) {
	s.flowCalls++
	return &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output": s.flowRows()}}, nil
}
func (s *extensionStock) InquireInvestorDailyForMarket(context.Context, string, string) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output": s.flowRows()}}, nil
}
func (s *extensionStock) InquireDailyItemChartPrice(context.Context, string, string, string, string, string) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output1": map[string]any{"stck_prpr": "99999"}, "output2": []any{map[string]any{"stck_bsop_date": "20260911", "stck_clpr": "900", "prdy_vrss": "100", "prdy_vrss_sign": "5", "acml_tr_pbmn": "12300000000"}}}}, nil
}
func (s *extensionStock) InquireInvestor(context.Context, string, string) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output": s.flowRows()}}, nil
}

type fixtureDailyPrices struct{}

func (fixtureDailyPrices) DailyStockPrices(context.Context, string) ([]dataapi.DailyStockRow, error) {
	return []dataapi.DailyStockRow{
		{Date: "20260911", ISIN: "KR1", Close: "100", Market: "KOSPI", Change: "0"}, {Date: "20260911", ISIN: "KR2", Close: "100", Market: "KOSPI", Change: "100"}, {Date: "20260911", ISIN: "KR3", Close: "100", Market: "KOSPI", Change: "-100"}, {Date: "20260911", ISIN: "KR4", Close: "100", Market: "KOSDAQ", Change: "0"}}, nil
}
func extensionReport(t *testing.T, s *extensionStock) *PremarketReport {
	t.Helper()
	return Collect(context.Background(), Deps{Stock: s, DailyPrices: fixtureDailyPrices{}, Clock: func() time.Time { return time.Date(2026, 9, 14, 8, 15, 0, 0, kst) }}, Options{StoreDir: t.TempDir(), NoSave: true})
}
func TestDomesticExtensionAlignedValues(t *testing.T) {
	s := newExtensionStock()
	r := extensionReport(t, s)
	in := r.DomesticInputs
	for key, want := range map[string]float64{"kospi_foreign_1d": 0, "kospi_foreign_3d": 3, "kospi_foreign_5d": 10, "kosdaq_institution_5d": -10, "kospi_turnover": 200, "kospi_turnover_mean20": 100, "kospi_turnover_ratio20": 2, "005930_close": 900, "005930_turnover": 123, "005930_foreign": 0, "005930_institution": -2, "kospi_advancers": 1, "kospi_decliners": 1, "kospi_unchanged": 1, "kosdaq_advancers": 0, "kospi_advancing_pct": 100.0 / 3} {
		o := in[key]
		if !o.available() || math.Abs(*o.Value-want) > 1e-9 {
			t.Errorf("%s = %+v; want %v", key, o, want)
		}
		if o.BusinessDate != "20260911" {
			t.Errorf("%s wrong date", key)
		}
	}
	if got := in["005930_close"].ChangePct; got == nil || *got != -10 {
		t.Errorf("unsigned fall must be -10%%: %v", got)
	}
	if s.flowCalls != 1 || s.calls["0001"] != 1 || s.calls["1001"] != 1 {
		t.Fatalf("duplicate collection: %d %+v", s.flowCalls, s.calls)
	}
	if in["kospi_foreign_5d"].ReferenceDate != s.dates[4] || in["kospi_foreign_5d"].SampleCount != 5 {
		t.Fatal("missing cumulative provenance")
	}
	if r.VUL.DScore != nil || r.VUL.SScore != nil {
		t.Fatal("new contextual data must not invent scores")
	}
	text := Render(r)
	if !strings.Contains(text, "3거래일 누적") || !strings.Contains(text, "공공데이터") {
		t.Fatal("domestic section missing")
	}
	b, e := json.Marshal(r)
	if e != nil || !strings.Contains(string(b), `"domestic_inputs"`) {
		t.Fatal("extension JSON missing")
	}
}
func TestCumulativeFlowDoesNotSkipMissingSession(t *testing.T) {
	s := newExtensionStock()
	s.gap = s.dates[1]
	r := extensionReport(t, s)
	if r.DomesticInputs["kospi_foreign_1d"].Value == nil {
		t.Fatal("valid zero lost")
	}
	if r.DomesticInputs["kospi_foreign_3d"].Value != nil || r.DomesticInputs["kospi_foreign_5d"].Value != nil {
		t.Fatal("must not replace missing day with older session")
	}
}
func TestTurnoverMeanRequiresTwentyPrecedingSessions(t *testing.T) {
	s := newExtensionStock()
	s.dates = s.dates[:20]
	r := extensionReport(t, s)
	if r.DomesticInputs["kospi_turnover_mean20"].Value != nil {
		t.Fatal("included current day or accepted insufficient history")
	}
}
func TestDailyRowConflictsAndSourceDates(t *testing.T) {
	rows := []map[string]any{{"stck_bsop_date": "20260911", "price": "1"}, {"stck_bsop_date": "20260911", "price": "2"}}
	if _, err := orderedDailyRows(rows, "20260914"); err == nil {
		t.Fatal("conflicting duplicate accepted")
	}
	rows[1]["price"] = "1"
	got, err := orderedDailyRows(rows, "20260914")
	if err != nil || len(got) != 1 {
		t.Fatal("identical duplicate not deduplicated")
	}
	rows = append(rows, map[string]any{"stck_bsop_date": "20260914", "price": "99"})
	got, err = orderedDailyRows(rows, "20260914")
	if err != nil || len(got) != 1 {
		t.Fatal("current session leaked")
	}
}
func TestBreadthRejectsPartialIdentityAndWrongDate(t *testing.T) {
	cases := [][]dataapi.DailyStockRow{
		{{Date: "20260914", ISIN: "KR1", Close: "100", Market: "KOSPI", Change: "0"}},
		{{Date: "20260911", ISIN: "KR1", Close: "100", Market: "KOSPI", Change: "NaN"}},
		{{Date: "20260911", ISIN: "KR1", Close: "100", Market: "KOSPI", Change: "1"}, {Date: "20260911", ISIN: "KR1", Close: "100", Market: "KOSPI", Change: "1"}},
	}
	for _, rows := range cases {
		if _, err := countBreadth(rows, "20260911"); err == nil {
			t.Fatal("invalid cross-section accepted")
		}
	}
}
