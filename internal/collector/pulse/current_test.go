package pulse

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/yahoo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type currentFixtureStock struct {
	observed   time.Time
	price      float64
	indexCalls atomic.Int32
}

func currentFixtureTimestamp(response *auth.RESTResponse, observed time.Time) *auth.RESTResponse {
	row := response.FirstRow("output")
	row["stck_bsop_date"] = observed.Format("20060102")
	row["stck_cntg_hour"] = observed.Format("150405")
	return response
}

func (stock *currentFixtureStock) MarketTime(context.Context) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output1": map[string]any{"today": stock.observed.Format("20060102")}}}, nil
}
func (stock *currentFixtureStock) InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error) {
	stock.indexCalls.Add(1)
	return currentFixtureTimestamp(idxResp(stock.price, 0, stock.price, stock.price, stock.price, 0, 0, 0), stock.observed), nil
}
func (stock *currentFixtureStock) InquireInvestorTimeByMarket(context.Context, string, string) (*auth.RESTResponse, error) {
	return currentFixtureTimestamp(flowResp(0, 0, 0), stock.observed), nil
}
func (stock *currentFixtureStock) CompProgramTradeToday(context.Context, string) (*auth.RESTResponse, error) {
	return currentFixtureTimestamp(&auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output": map[string]any{
		"bsop_hour": stock.observed.Format("150405"), "arbt_smtn_ntby_tr_pbmn": "0", "nabt_smtn_ntby_tr_pbmn": "0", "whol_smtn_ntby_tr_pbmn": "0",
	}}}, stock.observed), nil
}
func (*currentFixtureStock) ResolveVKOSPICode(context.Context, []string) (string, error) {
	return "0503", nil
}
func (stock *currentFixtureStock) InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error) {
	return currentFixtureTimestamp(idxResp(20, 0, 20, 20, 20, 0, 0, 0), stock.observed), nil
}
func (*currentFixtureStock) InquirePrice(context.Context, string) (*auth.RESTResponse, error) {
	return nil, errors.New("individual stock quotes not supplied by current fixture")
}
func (*currentFixtureStock) KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error) {
	return nil, errors.New("market-cap summary not supplied by current fixture")
}

type currentFixtureYahoo struct {
	observed   time.Time
	price      float64
	quoteCalls atomic.Int32
	chartCalls atomic.Int32
}

func (client *currentFixtureYahoo) GetQuotes(_ context.Context, symbols []string) (map[string]yahoo.Quote, error) {
	client.quoteCalls.Add(1)
	quotes := make(map[string]yahoo.Quote, len(symbols))
	for _, symbol := range symbols {
		quotes[symbol] = yahoo.Quote{Symbol: symbol, Price: client.price, PreviousClose: client.price, MarketTimeUnix: client.observed.Unix()}
	}
	return quotes, nil
}
func (client *currentFixtureYahoo) GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error) {
	client.chartCalls.Add(1)
	return []yahoo.DailyClose{
		{DateUnix: client.observed.Add(-2 * time.Hour).Unix(), Close: client.price - 20},
		{DateUnix: client.observed.Add(-time.Hour).Unix(), Close: client.price - 10},
		{DateUnix: client.observed.Unix(), Close: client.price},
	}, nil
}

func currentFixtureDeps(now time.Time, price float64) (Deps, *currentFixtureStock, *currentFixtureYahoo) {
	observed := now.Add(-30 * time.Second)
	stock := &currentFixtureStock{observed: observed, price: price}
	client := &currentFixtureYahoo{observed: observed, price: price}
	future := futureFake{resp: &auth.RESTResponse{Body: map[string]any{"rt_cd": "0",
		"output1": map[string]any{"futs_prpr": "2501", "futs_prdy_clpr": "2501", "futs_prdy_ctrt": "0", "mrkt_basis": "1", "stck_bsop_date": observed.Format("20060102"), "stck_cntg_hour": observed.Format("150405")},
		"output3": map[string]any{"bstp_cls_code": "2001", "bstp_nmix_prpr": "2500", "bstp_nmix_prdy_ctrt": "0"},
	}}}
	return Deps{Stock: stock, Future: future, Yahoo: client, Clock: func() time.Time { return now }}, stock, client
}

func expectCurrentFixtureJSON(report *Pulse) {
	GinkgoHelper()
	raw, err := json.Marshal(PulseToMap(report))
	Expect(err).NotTo(HaveOccurred())
	var data map[string]any
	Expect(json.Unmarshal(raw, &data)).To(Succeed())
	Expect(data).To(HaveKeyWithValue("comparison_mode", "CURRENT_ONLY"))
	Expect(data).To(HaveKeyWithValue("stored_count", float64(0)))
	for _, name := range []string{"kospi", "kosdaq"} {
		market := data[name].(map[string]any)
		for _, field := range []string{"move_1h", "move_2h", "flow_delta_prev", "flow_delta_first", "flow_delta_1h", "flow_delta_2h"} {
			Expect(market).To(HaveKeyWithValue(field, BeNil()), name+"."+field)
		}
		Expect(market).To(HaveKeyWithValue("change_pct", float64(0)))
		Expect(market["flow"]).To(SatisfyAll(HaveKeyWithValue("foreign", float64(0)), HaveKeyWithValue("source", "KIS inquire-investor-time-by-market")))
		Expect(market["index"]).To(SatisfyAll(HaveKeyWithValue("Source", "KIS inquire-index-price"), HaveKeyWithValue("last_ts", report.Now.Add(-30*time.Second).Format(time.RFC3339))))
	}
	windows := []any{data["kospi"].(map[string]any)["window"], data["kosdaq"].(map[string]any)["window"], data["usdkrw"]}
	windows = append(windows, data["macro"].([]any)...)
	for _, window := range windows {
		for _, field := range []string{"Move1hPct", "Move2hPct", "Ref1hTS", "Ref2hTS"} {
			Expect(window).To(HaveKeyWithValue(field, BeNil()))
		}
		Expect(window).To(HaveKeyWithValue("ChangePct", float64(0)))
	}
	Expect(data["program_trade"]).To(SatisfyAll(HaveKeyWithValue("kospi_delta", BeNil()), HaveKeyWithValue("kosdaq_delta", BeNil())))
	Expect(data).To(SatisfyAll(HaveKeyWithValue("basis_delta_1h", BeNil()), HaveKeyWithValue("basis_delta_2h", BeNil())))
	Expect(strings.Join(report.Analysis, "\n")).NotTo(ContainSubstring("1h"))
	Expect(strings.Join(report.Analysis, "\n")).NotTo(ContainSubstring("2h"))
	Expect(report.Assessment.Direction).To(Equal("NOT_EVALUATED"))
	Expect(report.Assessment.ExternalMacro).To(Equal("NOT_EVALUATED"))
}

var _ = Describe("Current-only pulse collection", func() {
	var now time.Time
	var dir string
	BeforeEach(func() {
		now = time.Date(2026, 10, 15, 13, 0, 0, 0, kstLocation)
		dir = GinkgoT().TempDir()
		GinkgoT().Setenv("OFFICIAL_EVENTS_FILE", filepath.Join(dir, "absent-official-events.json"))
	})

	Context("when observation history must stay unchanged", func() {
		It("ignores corrupt history and fetches new values on repeated calls without reading or appending comparisons", func() {
			path := pulseFilePath(dir, "20261015")
			before := []byte("{corrupt observation history}\n")
			Expect(os.WriteFile(path, before, 0o600)).To(Succeed())
			for iteration, price := range []float64{2500, 2525} {
				callTime := now.Add(time.Duration(iteration) * time.Minute)
				deps, stock, client := currentFixtureDeps(callTime, price)
				report := Collect(context.Background(), deps, Options{StoreDir: dir, CurrentOnly: true})
				Expect(report.KOSPI.Index.Price).To(Equal(price))
				Expect(report.KOSPI.IntradayWin.Current).To(Equal(price))
				Expect(report.Now).To(Equal(callTime))
				Expect(report.KOSPI.Index.LastTS).To(Equal(callTime.Add(-30 * time.Second)))
				Expect(report.KOSPI.Index.FetchedAt).To(Equal(callTime))
				Expect(stock.indexCalls.Load()).To(BeNumerically(">=", 2))
				Expect(client.quoteCalls.Load()).To(Equal(int32(1)))
				Expect(client.chartCalls.Load()).To(Equal(int32(len(macroSymbols))))
				Expect(report.Errors).NotTo(HaveKey("store_load"))
				Expect(report.Errors).NotTo(HaveKey("store_append"))
				Expect(report.Saved).To(BeFalse())
				Expect(report.PrevTS).To(BeNil())
				expectCurrentFixtureJSON(report)
				after, err := os.ReadFile(path)
				Expect(err).NotTo(HaveOccurred())
				Expect(after).To(Equal(before))
			}
		})

		It("does not create a JSONL observation file when none exists", func() {
			deps, _, _ := currentFixtureDeps(now, 2500)
			report := Collect(context.Background(), deps, Options{StoreDir: dir, CurrentOnly: true})
			Expect(report.KOSPI.Index.OK).To(BeTrue())
			Expect(report.CurrentOnly).To(BeTrue())
			Expect(report.Saved).To(BeFalse())
			Expect(report.StoredCount).To(BeZero())
			_, err := os.Stat(pulseFilePath(dir, "20261015"))
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})

	Context("when the normal CLI collector retains comparisons", func() {
		It("appends history and keeps provider intervals until explicitly converted to current-only output", func() {
			for _, hours := range []int{2, 1} {
				timestamp := now.Add(-time.Duration(hours) * time.Hour)
				observed := timestamp.Add(-30 * time.Second)
				Expect(AppendRecord(dir, "20261015", PulseRecord{
					SchemaVersion: 2, BusinessDate: "20261015", TS: timestamp,
					KOSPIFlow:    FlowSnapshot{OK: true, LastTS: observed, Foreign: -100, EtcForeignOK: true},
					KOSDAQFlow:   FlowSnapshot{OK: true, LastTS: observed, Foreign: -100, EtcForeignOK: true},
					KOSPIProgram: ProgramTradeSnapshot{OK: true, LastTS: observed, Total: -100},
				})).To(Succeed())
			}
			deps, _, _ := currentFixtureDeps(now, 2500)
			report := Collect(context.Background(), deps, Options{StoreDir: dir})
			Expect(report.CurrentOnly).To(BeFalse())
			Expect(report.Saved).To(BeTrue())
			Expect(report.StoredCount).To(Equal(3))
			Expect(PulseToMap(report)).NotTo(HaveKey("comparison_mode"))
			Expect(report.KOSPI.FlowDeltaPrev).NotTo(BeNil())
			Expect(report.KOSPI.FlowDelta1h).NotTo(BeNil())
			Expect(report.KOSPI.FlowDelta2h).NotTo(BeNil())
			Expect(report.KOSPIProgramDelta1h).NotTo(BeNil())
			for _, window := range []Window{report.KOSPI.IntradayWin, report.KOSDAQ.IntradayWin, report.USDKRW, report.Macro[0]} {
				Expect(window.Move1hPct).NotTo(BeNil())
				Expect(window.Move2hPct).NotTo(BeNil())
				Expect(*window.Move1hPct).To(BeNumerically(">", 0))
				Expect(*window.Move2hPct).To(BeNumerically(">", 0))
				Expect(window.Ref1hTS).NotTo(BeNil())
				Expect(window.Ref2hTS).NotTo(BeNil())
			}
			records, err := LoadRecords(dir, "20261015")
			Expect(err).NotTo(HaveOccurred())
			Expect(records).To(HaveLen(3))
			Expect(records[2].KOSPIIndex.Price).To(Equal(2500.0))
			before, err := os.ReadFile(pulseFilePath(dir, "20261015"))
			Expect(err).NotTo(HaveOccurred())
			report.SetCurrentOnly()
			expectCurrentFixtureJSON(report)
			after, err := os.ReadFile(pulseFilePath(dir, "20261015"))
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(before))
		})
	})
})
