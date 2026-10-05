package pulse

import (
	"context"
	"encoding/json"
	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/external/yahoo"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func qualityNow() time.Time { return time.Date(2026, 9, 15, 10, 0, 0, 0, kstLocation) }
func TestCapturedKISQuality(t *testing.T) {
	b, e := os.ReadFile("testdata/quality_20260915.json")
	if e != nil {
		t.Fatal(e)
	}
	var bodies map[string]map[string]any
	if e = json.Unmarshal(b, &bodies); e != nil {
		t.Fatal(e)
	}
	idx, e := collectIndex(context.Background(), testStock{indexResp: map[string]*auth.RESTResponse{"0001": {Body: bodies["index"]}}}, "0001", qualityNow())
	if e != nil || idx.Price != 6589.52 || !idx.LastTS.IsZero() || idx.Freshness != "UNKNOWN" {
		t.Fatalf("unexpected index: %+v %v", idx, e)
	}
	f, e := collectFlow(context.Background(), testStock{flowResp: map[string]*auth.RESTResponse{"KSP": {Body: bodies["flow"]}}}, "KSP", "0001", qualityNow())
	if e != nil || !f.OK || f.Foreign != -13790.31 || f.EtcForeignOK {
		t.Fatalf("flow: %+v %v", f, e)
	}
	raw, _ := json.Marshal(f)
	if !strings.Contains(string(raw), `"EtcForeign":null`) {
		t.Fatal(string(raw))
	}
	fut, e := collectIndexFuture(context.Background(), futureFake{&auth.RESTResponse{Body: bodies["wrong_spot_future"]}}, "20260915", "KOSDAQ150", qualityNow())
	if e != nil || !fut.OK || fut.SpotOK || fut.SpotCode != "2001" || fut.BasisMatch {
		t.Fatalf("wrong underlying accepted: %+v %v", fut, e)
	}
	raw, _ = json.Marshal(fut)
	if !strings.Contains(string(raw), `"spot_price":null`) {
		t.Fatal(string(raw))
	}
}
func TestMissingAndBusinessErrorsAreNotValidZero(t *testing.T) {
	for _, body := range []map[string]any{{"rt_cd": "1", "output": map[string]any{"frgn_ntby_tr_pbmn": "0"}}, {"rt_cd": "0", "output": map[string]any{"aspr_hour": "100000"}}} {
		f, e := collectFlow(context.Background(), testStock{flowResp: map[string]*auth.RESTResponse{"KSP": {Body: body}}}, "KSP", "0001", qualityNow())
		if e == nil || f.OK {
			t.Fatal("invalid flow accepted")
		}
	}
	r := flowResp(0, 0, 0)
	f, e := collectFlow(context.Background(), testStock{flowResp: map[string]*auth.RESTResponse{"KSP": r}}, "KSP", "0001", qualityNow())
	if e != nil || !f.OK || f.Foreign != 0 {
		t.Fatal("observed zero lost")
	}
}
func TestWindowUsesBarEndpointAndBoundedAnchor(t *testing.T) {
	now := qualityNow()
	q := yahoo.Quote{Price: 90, PreviousClose: 100, MarketTimeUnix: now.Add(-2 * time.Hour).Unix()}
	bars := []yahoo.DailyClose{{DateUnix: now.Add(-time.Hour).Unix(), Close: 100}, {DateUnix: now.Unix(), Close: 110}, {DateUnix: now.Add(time.Hour).Unix(), Close: 200}}
	w := buildWindow("NQ=F", "NQ", q, bars, now)
	if w.Current != 110 || !w.LastTS.Equal(now) || w.Move1hPct == nil || *w.Move1hPct < 9.99 || w.Move2hPct != nil {
		t.Fatalf("mixed endpoint: %+v", w)
	}
	w = buildWindow("NQ=F", "NQ", q, []yahoo.DailyClose{{DateUnix: now.Add(-20 * time.Minute).Unix(), Close: 100}, {DateUnix: now.Unix(), Close: 110}}, now)
	if w.Move1hPct != nil || w.Move2hPct != nil {
		t.Fatal("20 minutes promoted to 1h/2h")
	}
	freshness, _, _ := DetermineFreshness("KRX", now.Add(time.Second), now, false)
	if freshness != "UNKNOWN" {
		t.Fatal("future timestamp accepted")
	}
}
func TestFlowAnchorRequiresValidObservationTimes(t *testing.T) {
	now := qualityNow()
	cur := FlowSnapshot{OK: true, Foreign: 1000, LastTS: now}
	records := []PulseRecord{{TS: now.Add(-187 * time.Minute), BusinessDate: "20260915", KOSPIFlow: FlowSnapshot{OK: false, Foreign: 10}}}
	a, b := computeFlowDeltas(records, now, cur, 100, "kospi")
	if a != nil || b != nil {
		t.Fatal("distant invalid anchor accepted")
	}
	records[0].TS = now.Add(-time.Hour)
	records[0].KOSPIFlow = FlowSnapshot{OK: true, Foreign: 400, LastTS: now.Add(-time.Hour)}
	a, _ = computeFlowDeltas(records, now, cur, 100, "kospi")
	if a == nil || a.Foreign != 600 || a.Elapsed != 60 {
		t.Fatalf("valid interval rejected %+v", a)
	}
	records[0].KOSPIFlow.LastTS = time.Time{}
	a, _ = computeFlowDeltas(records, now, cur, 100, "kospi")
	if a != nil {
		t.Fatal("undated flow accepted")
	}
}
func TestMissingAssessmentAndJSON(t *testing.T) {
	p := &Pulse{Now: qualityNow()}
	p.Analysis = Analyze(p)
	a := p.Assessment
	if a.Direction != "NOT_EVALUATED" || a.Stress != "NOT_EVALUATED" || a.InternalBreadth != "NOT_EVALUATED" || a.ExternalMacro != "NOT_EVALUATED" {
		t.Fatalf("missing input inferred: %+v", a)
	}
	m := PulseToMap(p)
	if m["kospi"].(map[string]any)["price"] != nil || m["assessment"] == nil {
		t.Fatal("JSON missing metadata")
	}
	raw, e := json.Marshal(m)
	if e != nil || !strings.Contains(string(raw), `"confidence":null`) {
		t.Fatalf("%s %v", raw, e)
	}
}
func TestConditionsNeverBecomeOfficialEvents(t *testing.T) {
	t.Setenv("OFFICIAL_EVENTS_FILE", filepath.Join(t.TempDir(), "absent.json"))
	t.Setenv("OFFICIAL_EVENT_KOSPI_SIDECAR_SELL", "09:00:00")
	now := qualityNow()
	records := []PulseRecord{{TS: now.Add(-6 * time.Minute), Safety: MarketSafety{Devices: []SafetyDeviceStatus{{Market: "KOSPI", Device: "SIDECAR_SELL", State: "CONDITION_OBSERVED", ConditionObservedAt: "09:54:00"}}}}}
	f := IndexFutureSnapshot{OK: true, Freshness: "FRESH", ChangePct: -6}
	s := buildMarketSafety(now, "20260915", IndexLevel{}, IndexLevel{}, f, IndexFutureSnapshot{}, records)
	for _, d := range s.Devices {
		if d.TriggeredAt != "" || d.ReleasedAt != "" || d.Verification == "OFFICIAL" {
			t.Fatalf("invented trigger: %+v", d)
		}
	}
}
func TestExplicitTriggerDoesNotInventRelease(t *testing.T) {
	p := filepath.Join(t.TempDir(), "events.json")
	t.Setenv("OFFICIAL_EVENTS_FILE", p)
	os.WriteFile(p, []byte(`[{"market":"KOSPI","device":"SIDECAR_SELL","triggered_at":"09:00:00","business_date":"20260915","source":"https://example.test/fixture"}]`), 0600)
	s := buildMarketSafety(qualityNow(), "20260915", IndexLevel{}, IndexLevel{}, IndexFutureSnapshot{OK: true}, IndexFutureSnapshot{}, nil)
	for _, d := range s.Devices {
		if d.Device == "SIDECAR_SELL" && (d.State != "TRIGGERED" || d.ReleasedAt != "") {
			t.Fatalf("invented release %+v", d)
		}
	}
	if len(LoadOfficialEvents("20260916")) != 0 {
		t.Fatal("previous-day event reused")
	}
}
func TestHistorySortAndCorruption(t *testing.T) {
	dir := t.TempDir()
	now := qualityNow()
	AppendRecord(dir, "20260915", PulseRecord{TS: now})
	AppendRecord(dir, "20260915", PulseRecord{TS: now.Add(-time.Hour)})
	r, e := LoadRecords(dir, "20260915")
	if e != nil || !r[0].TS.Before(r[1].TS) {
		t.Fatal("history unsorted")
	}
	f, _ := os.OpenFile(pulseFilePath(dir, "20260915"), os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString("broken\n")
	f.Close()
	if _, e = LoadRecords(dir, "20260915"); e == nil {
		t.Fatal("corruption hidden")
	}
}
func TestContractRollNeverProducesBasisDelta(t *testing.T) {
	now := qualityNow()
	cur := IndexFutureSnapshot{OK: true, Code: "DEC", Basis: 3, Alignment: "ALIGNED"}
	r := []PulseRecord{{TS: now.Add(-time.Hour), BusinessDate: "20260915", KOSPI200Future: IndexFutureSnapshot{OK: true, Code: "SEP", Basis: 1, Alignment: "ALIGNED"}}}
	if computeBasisDelta(r, now, cur, 1) != nil {
		t.Fatal("cross-contract basis delta")
	}
}

func TestPastObservationDateIsNeverRefreshedByCollection(t *testing.T) {
	now := qualityNow()
	r := idxResp(100, 0, 100, 100, 100, 0, 1, 1)
	row := r.FirstRow("output")
	row["stck_bsop_date"] = "20260914"
	row["bsop_hour"] = "153000"
	got, e := collectIndex(context.Background(), testStock{indexResp: map[string]*auth.RESTResponse{"0001": r}}, "0001", now)
	if e != nil || got.LastTS.In(kstLocation).Format("20060102") != "20260914" || got.Freshness != "STALE" {
		t.Fatalf("past timestamp refreshed %+v %v", got, e)
	}
}
