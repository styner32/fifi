package premarket

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/external/kofia"
	"github.com/fifi/internal/external/yahoo"
)

type regressionStock struct {
	mu       sync.Mutex
	calls    map[string]int
	flowDate string
}

func (s *regressionStock) called(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[key]++
}
func (s *regressionStock) MarketTime(context.Context) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"output1": map[string]any{"today": "20260914", "date1": "20260914", "date2": "20260911", "date3": "20260910"}}}, nil
}
func (s *regressionStock) InquireIndexDailyPrice(_ context.Context, code, date string) ([]map[string]any, error) {
	s.called(code)
	price, pct := "6,909.91", "-1.76"
	if code == "1001" {
		price, pct = "820.64", "-1.95"
	}
	return []map[string]any{{"stck_bsop_date": "20260914", "bstp_nmix_prpr": "9999", "bstp_nmix_prdy_ctrt": "0"}, {"stck_bsop_date": "20260911", "bstp_nmix_prpr": price, "bstp_nmix_prdy_ctrt": pct}}, nil
}
func (s *regressionStock) InquireVKOSPIDailyPrice(context.Context, string, string) ([]map[string]any, error) {
	s.called("vkospi")
	return []map[string]any{{"stck_bsop_date": "20260911", "bstp_nmix_prpr": "46.28", "bstp_nmix_prdy_ctrt": "-2.12"}}, nil
}

func (*regressionStock) InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"output": map[string]any{"bstp_nmix_prpr": "6,909.91"}}}, nil
}
func (*regressionStock) ResolveVKOSPICode(context.Context, []string) (string, error) {
	return "0503", nil
}
func (*regressionStock) InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"output": map[string]any{"bstp_nmix_prpr": "46.28"}}}, nil
}
func (*regressionStock) MarketFunds(context.Context, string) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"output": map[string]any{"bsop_date": "20260911", "crdt_loan_rmnd": "335,000", "cust_dpmn_amt": "1,067,000"}}}, nil
}
func (s *regressionStock) InquireInvestorDailyByMarket(_ context.Context, date string) (*auth.RESTResponse, error) {
	s.flowDate = date
	return &auth.RESTResponse{Body: map[string]any{"output": map[string]any{"stck_bsop_date": "20260911", "frgn_ntby_tr_pbmn": "0"}}}, nil
}

type regressionYahoo struct {
	fail       string
	mu         sync.Mutex
	quoteCalls int
}

func (y *regressionYahoo) GetQuotes(context.Context, []string) (map[string]yahoo.Quote, error) {
	y.mu.Lock()
	y.quoteCalls++
	y.mu.Unlock()
	ts := time.Date(2026, 9, 14, 8, 10, 0, 0, kst).Unix()
	return map[string]yahoo.Quote{"NQ=F": {Price: 29000, PreviousClose: 29325, MarketTimeUnix: ts}, "KRW=X": {Price: 1342.9, PreviousClose: 1341.05, MarketTimeUnix: ts}}, nil
}
func (y *regressionYahoo) GetChartHistory(_ context.Context, symbol, _, _ string) ([]yahoo.DailyClose, error) {
	if symbol == y.fail {
		return nil, errors.New("fixture outage")
	}
	ret := map[string]float64{"SKHY": .94, "^SOX": 1.81, "MU": -.22, "NVDA": -.03, "ASML": .64}
	v := 100 * (1 + ret[symbol]/100)
	loc, _ := time.LoadLocation("America/New_York")
	return []yahoo.DailyClose{{DateUnix: time.Date(2026, 9, 10, 9, 30, 0, 0, loc).Unix(), Close: 100}, {DateUnix: time.Date(2026, 9, 11, 9, 30, 0, 0, loc).Unix(), Close: v}, {DateUnix: time.Date(2026, 9, 14, 9, 30, 0, 0, loc).Unix(), Close: 9999}}, nil
}

type regressionKofia struct{}

func (regressionKofia) GetMarketFundsForDate(context.Context, string) (*kofia.MarketFundsRow, error) {
	return &kofia.MarketFundsRow{Date: "20260910", MarginReceivableMln: 10000, ForcedSellAmountMln: 0, ForcedSellRatioPct: 0}, nil
}
func fixedReport(t *testing.T, fail string) *PremarketReport {
	t.Helper()
	s := &regressionStock{}
	y := &regressionYahoo{fail: fail}
	r := Collect(context.Background(), Deps{Stock: s, Yahoo: y, KOFIA: regressionKofia{}, Clock: func() time.Time { return time.Date(2026, 9, 14, 8, 15, 0, 0, kst) }}, Options{StoreDir: t.TempDir(), NoSave: true})
	if s.flowDate != "20260911" {
		t.Errorf("flow queried for %s", s.flowDate)
	}
	for _, key := range []string{"0001", "1001", "vkospi"} {
		if s.calls[key] != 1 {
			t.Errorf("%s fetched %d times", key, s.calls[key])
		}
	}
	if y.quoteCalls != 1 {
		t.Error("duplicate quote fetch")
	}
	return r
}

func TestFixedSeptember14Premarket(t *testing.T) {
	r := fixedReport(t, "")
	if r.Tier2.VKOSPI == nil || *r.Tier2.VKOSPI != 46.28 {
		t.Fatalf("VKOSPI=%v", r.Tier2.VKOSPI)
	}
	if math.Abs(*r.Tier2.SigmaDaily-46.28/math.Sqrt(252)) > 1e-10 {
		t.Fatal("invalid sigma")
	}
	if r.Tier2.CreditLoanBalanceEok == nil || *r.Tier2.CreditLoanBalanceEok != 335000 {
		t.Fatal("credit parse/unit failure")
	}
	if r.Tier2.CustomerDepositEok == nil || *r.Tier2.CustomerDepositEok != 1067000 {
		t.Fatal("deposit parse/unit failure")
	}
	if r.Tier2.ForcedSellAmountEok == nil || *r.Tier2.ForcedSellAmountEok != 0 {
		t.Fatal("observed zero lost")
	}
	if r.Tier1.SemiComposite == nil || math.Abs(*r.Tier1.SemiComposite-.808) > 1e-9 {
		t.Fatal("weighted return differs from 0.808%")
	}
	if r.Inputs["margin"].BusinessDate != "20260910" || *r.Tier2.MarginReceivableEok != 100 {
		t.Fatal("KOFIA date/unit lost")
	}
	if r.Tier1.SKHYPremium != nil || r.Tier1.NDFGap != nil || r.Tier1.Divergence != nil {
		t.Fatal("unverified calculation leaked")
	}
	if r.VUL.ConfidencePct != nil || !r.VUL.Suppressed {
		t.Fatal("invalid confidence/grade")
	}
	out := Render(r)
	for _, want := range []string{"6,909.91", "-1.76%", "2026-09-11 일별 종가", "46.28", "±2.92%", "+0.81%", "SKHY 35%", "순매수·순매도 균형"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, "2026-09-14 종가") || strings.Contains(out, "2026-09-14 06:00") {
		t.Fatal("invented close date")
	}
	t.Logf("VKOSPI %.2f; sigma %.4f; credit %.0f; deposit %.0f; SEMI %.3f; grade %s", *r.Tier2.VKOSPI, *r.Tier2.SigmaDaily, *r.Tier2.CreditLoanBalanceEok, *r.Tier2.CustomerDepositEok, *r.Tier1.SemiComposite, r.VUL.OverallGrade)
}

func TestPartialSemiconductorDataIsNotReweighted(t *testing.T) {
	r := fixedReport(t, "MU")
	if r.Tier1.SemiComposite != nil || math.Abs(r.Tier1.SemiWeightCoveragePct-85) > 1e-9 {
		t.Fatal("partial basket masqueraded as full basket")
	}
	if r.Inputs["semi_composite"].Reason != "MISSING_OR_MISALIGNED_MEMBERS" {
		t.Fatal("missing reason")
	}
	if r.Errors["MU"] == "" {
		t.Fatal("source error lost")
	}
}

func TestCoverageCountsObservationsNotMarketAlerts(t *testing.T) {
	r := fixedReport(t, "")
	expected := 0
	for _, o := range r.Inputs {
		if !o.available() {
			expected++
		}
	}
	if expected != r.VUL.MissingCount {
		t.Fatal("bad missing count")
	}
	old := r.VUL.CoveragePct
	r.Tier1.QualityFlags = append(r.Tier1.QualityFlags, "TEST_MARKET_ALERT")
	v := calculateVulnerabilityMatrix(r.Tier1, r.Tier2, r.Inputs)
	if v.CoveragePct != old {
		t.Fatal("market alert changed coverage")
	}
}

func TestCompletedUSSessionAndDST(t *testing.T) {
	for _, month := range []time.Month{time.January, time.July} {
		loc, _ := time.LoadLocation("America/New_York")
		start := time.Date(2026, month, 13, 9, 30, 0, 0, loc)
		h := []yahoo.DailyClose{{DateUnix: start.AddDate(0, 0, -1).Unix(), Close: 100}, {DateUnix: start.Unix(), Close: 101}}
		before := time.Date(2026, month, 13, 15, 59, 0, 0, loc)
		o := completedUSClose(h, "EWY", before, before)
		if o.BusinessDate != start.AddDate(0, 0, -1).Format("20060102") {
			t.Fatal("partial daily bar accepted")
		}
		after := before.Add(2 * time.Minute)
		o = completedUSClose(h, "EWY", after, after)
		if o.BusinessDate != start.Format("20060102") {
			t.Fatal("exchange trading date shifted to KST")
		}
	}
}

func TestDatedRowsRejectPreopenFutureAndUndated(t *testing.T) {
	rows := []map[string]any{{"date": "20260914", "v": "0"}, {"date": "20260910", "v": "4"}, {"date": "20260911", "v": "0"}, {"v": "9"}, {"date": "20260915", "v": "9"}}
	row := datedRow(rows, "date", "20260914", "20260911")
	if row == nil || row["v"] != "0" {
		t.Fatal("valid prior-session zero rejected")
	}
	if datedRow(rows, "date", "20260914", "20260909") != nil {
		t.Fatal("mismatched business date accepted")
	}
}

func TestInvalidNumbersAndBusinessErrors(t *testing.T) {
	for _, v := range []any{"", "NaN", "Inf", "bad", math.Inf(-1)} {
		if _, ok := num(map[string]any{"v": v}, "v"); ok {
			t.Fatalf("accepted %v", v)
		}
	}
	if v, ok := num(map[string]any{"v": "1,067,000"}, "v"); !ok || v != 1067000 {
		t.Fatal("comma parsing")
	}
	if firstRow(&auth.RESTResponse{Body: map[string]any{"rt_cd": "1", "output": map[string]any{"v": "42"}}}, "output") != nil {
		t.Fatal("business error accepted")
	}
	if fmtComma(999.999, 2) != "1,000.00" {
		t.Fatal("rounding carry failed")
	}
}

func TestStoreExcludesLegacyAndDeduplicatesSourceDates(t *testing.T) {
	dir := t.TempDir()
	legacy := []byte(`{"records":[{"date":"20260910","vkospi":99}]}`)
	if err := os.WriteFile(filepath.Join(dir, "store.json"), legacy, 0600); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)
	if len(s.Data.Records) != 0 {
		t.Fatal("unverified legacy history imported")
	}
	now := time.Date(2026, 9, 14, 8, 0, 0, 0, kst)
	add := func(report, source string, v float64) {
		s.UpsertRecord(DailyRecord{Date: report, ObservedAt: now, Inputs: map[string]Observation{"credit": {Value: ptr(v), BusinessDate: source, Status: "LAGGED_CONTEXT_ONLY", Session: "DAILY_STATISTIC"}}})
	}
	add("20260910", "20260909", 100)
	add("20260911", "20260909", 100)
	add("20260912", "20260910", 50)
	add("20260912", "20260910", 0)
	add("20260915", "20260911", 999)
	h := s.History("credit", "20260911", "20260914", 60)
	if len(h) != 2 || h[0] != 0 || h[1] != 100 {
		t.Fatalf("history %#v", h)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "store.json")); string(got) != string(legacy) {
		t.Fatal("legacy evidence overwritten")
	}
	if NewStore(dir).Err != nil {
		t.Fatal("v2 round trip failed")
	}
	if err := os.WriteFile(filepath.Join(dir, "store.v2.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := NewStore(dir).Save(); err == nil {
		t.Fatal("corrupt history overwritten")
	}
}

func TestPercentilesRequireFullDatedHistory(t *testing.T) {
	s := NewStore(t.TempDir())
	in := emptyInputs(time.Now())
	in["credit"] = Observation{Value: ptr(250), BusinessDate: "20260911", Status: "VALID"}
	in["vkospi"] = Observation{Value: ptr(250), BusinessDate: "20260911", Status: "VALID", Session: "DAILY_CLOSE"}
	date, _ := time.Parse("20060102", "20260910")
	for n := 0; n < 250; n++ {
		d := date.AddDate(0, 0, -n).Format("20060102")
		s.UpsertRecord(DailyRecord{Date: d, ObservedAt: date, Inputs: map[string]Observation{
			"credit": {Value: ptr(float64(n)), BusinessDate: d, Status: "VALID"},
			"vkospi": {Value: ptr(float64(n + 1)), BusinessDate: d, Status: "VALID", Session: "DAILY_CLOSE"},
		}})
		if n == 58 {
			tier := collectTier2(in, s, "20260914")
			if tier.CreditLoanPctile != nil || tier.VKOSPIPctile250d != nil || tier.CreditSampleCount != 59 {
				t.Fatal("partial history labelled as full percentile")
			}
		}
	}
	tier := collectTier2(in, s, "20260914")
	if tier.CreditSampleCount != 60 || tier.VKOSPISampleCount != 250 || *tier.CreditLoanPctile != 100 || *tier.VKOSPIPctile250d != 100 {
		t.Fatal("full history percentile failed")
	}
}

func TestProviderBarDoesNotInventSettlementOrExchangeDate(t *testing.T) {
	now := time.Date(2026, 9, 14, 8, 15, 0, 0, kst)
	h := []yahoo.DailyClose{{DateUnix: now.Add(-72 * time.Hour).Unix(), Close: 100}, {DateUnix: now.Add(-48 * time.Hour).Unix(), Close: 102}, {DateUnix: now.Add(-time.Hour).Unix(), Close: 999}}
	o := completedProviderBar(h, "CL=F", now, now)
	if o.Value == nil || *o.Value != 102 || o.BusinessDate != "" || o.Session != "PROVIDER_DAILY" {
		t.Fatal("partial bar or invented exchange date")
	}
	if math.Abs(*o.ChangePct-2) > 1e-9 {
		t.Fatal("wrong provider change")
	}
	if !strings.Contains(observationRef(o), "정산가 미검증") {
		t.Fatal("provider bar masquerades as settlement")
	}
}

func TestAfternoonRunKeepsPremarketCashSessions(t *testing.T) {
	r := Collect(context.Background(), Deps{Stock: &regressionStock{}, Yahoo: &regressionYahoo{}, Clock: func() time.Time { return time.Date(2026, 9, 14, 16, 0, 0, 0, kst) }}, Options{NoSave: true, StoreDir: t.TempDir()})
	if r.Inputs["kospi"].BusinessDate != "20260911" || r.Inputs["^GSPC"].BusinessDate != "20260911" {
		t.Fatal("afternoon values replaced premarket cash closes")
	}
}

func reportMap(t *testing.T, r *PremarketReport) map[string]any {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRealRESTResponseReader(t *testing.T) {
	r := &auth.RESTResponse{Body: map[string]any{"output": map[string]any{"bstp_nmix_prpr": "46.28"}}}
	row := firstRow(r, "output")
	if row == nil {
		t.Fatal("normal RESTResponse was discarded")
	}
}

func TestMissingInputsNeverProduceGreen(t *testing.T) {
	r := Collect(context.Background(), Deps{Clock: func() time.Time { return time.Date(2026, 9, 14, 8, 15, 0, 0, time.FixedZone("KST", 32400)) }}, Options{StoreDir: t.TempDir(), NoSave: true})
	if !r.VUL.Suppressed {
		t.Fatal("missing inputs produced an unsuppressed grade")
	}
	for _, stale := range []string{"7/29", "7/30", "7/31", "5일 연속 감소", "-11.2%", "-13.8%", "NDF 0.0"} {
		if strings.Contains(Render(r), stale) {
			t.Errorf("fabricated output: %s", stale)
		}
	}
}
