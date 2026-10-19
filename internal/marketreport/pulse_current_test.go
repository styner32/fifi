package marketreport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/kst"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func comparisonPulseFixture(at time.Time, price float64) *pulse.Pulse {
	move, ref := 2.0, at.Add(-time.Hour)
	window := pulse.Window{Symbol: "^KS11", Label: "fixture", Current: price, ChangeOK: true, ChangePct: 1, PrevClose: price / 1.01, OK: true, Source: "fixture", LastTS: at.Add(-time.Minute), FetchedAt: at, Freshness: "FRESH", Move1hPct: &move, Move2hPct: &move, Ref1hTS: &ref, Ref2hTS: &ref}
	market := pulse.Market{Name: "KOSPI", Index: pulse.IndexLevel{Price: price, ChangePct: 1, Source: "fixture", LastTS: window.LastTS, FetchedAt: at, OK: true, Freshness: "FRESH"}, IntradayWin: window, Flow: pulse.FlowSnapshot{OK: true, Foreign: 0, Source: "fixture", FetchedAt: at}, FlowDeltaPrev: &pulse.FlowDelta{Foreign: 10}, FlowDeltaAnchor: &pulse.FlowDelta{Foreign: 10}, FlowDelta1h: &pulse.FlowDelta{Foreign: 10}, FlowDelta2h: &pulse.FlowDelta{Foreign: 10}}
	fx, macro := window, window
	fx.Symbol, fx.Current = "KRW=X", 1400
	macro.Symbol = "NQ=F"
	return &pulse.Pulse{Now: at, Date: at.In(kst.Location).Format("20060102"), BusinessDate: "20261015", KOSPI: market, KOSDAQ: market, USDKRW: fx, Macro: []pulse.Window{macro}, KOSPIProgramDelta: &pulse.ProgramTradeDelta{Total: 10}, KOSDAQProgramDelta: &pulse.ProgramTradeDelta{Total: 10}, BasisDelta1h: &pulse.BasisDelta{Value: 1}, BasisDelta2h: &pulse.BasisDelta{Value: 1}, PrevTS: &ref, StoredCount: 12, Saved: true, Errors: map[string]string{}}
}

var _ = Describe("Current-only pulse reports", func() {
	var config Config
	var now time.Time
	var output string
	BeforeEach(func() {
		dir := GinkgoT().TempDir()
		now = time.Date(2026, 10, 15, 13, 0, 0, 0, kst.Location)
		config = Config{PulseDir: filepath.Join(dir, "cli"), LockFile: filepath.Join(dir, "market.lock"), Clock: func() time.Time { return now }}
		output = filepath.Join(dir, "mcp")
	})
	Context("when saving an MCP current observation", func() {
		It("saves fresh standalone JSON/Markdown without changing CLI reports or adding observations", func() {
			Expect(pulse.AppendRecord(config.PulseDir, "20261015", pulse.PulseRecord{SchemaVersion: 2, TS: now.Add(-time.Hour)})).To(Succeed())
			historyPath := filepath.Join(config.PulseDir, "pulse_20261015.jsonl")
			before, err := os.ReadFile(historyPath)
			Expect(err).NotTo(HaveOccurred())
			cliMD := filepath.Join(config.PulseDir, "pulse_20261015.md")
			Expect(os.WriteFile(cliMD, []byte("existing CLI report"), 0600)).To(Succeed())
			calls := 0
			service := NewWithCollectors(config, Collectors{Pulse: func(_ context.Context, options pulse.Options) (*pulse.Pulse, error) {
				Expect(options.CurrentOnly).To(BeTrue())
				Expect(options.NoHistory).To(BeTrue())
				Expect(options.NoSave).To(BeTrue())
				Expect(options.StoreDir).To(Equal(output))
				calls++
				return comparisonPulseFixture(now.Add(time.Duration(calls-1)*time.Minute), 2500+float64(calls)), nil
			}})
			for call := 1; call <= 2; call++ {
				result, err := service.RunPulse(context.Background(), pulse.Options{CurrentOnly: true, StoreDir: output})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Envelope.Artifacts).To(HaveLen(2))
				Expect(result.Envelope.Report).To(HaveKeyWithValue("comparison_mode", "CURRENT_ONLY"))
				kospi := result.Envelope.Report["kospi"].(map[string]any)
				Expect(kospi["price"]).To(Equal(json.Number(strconv.Itoa(2500 + call))))
				for _, name := range []string{"move_1h", "move_2h", "flow_delta_prev", "flow_delta_first", "flow_delta_1h", "flow_delta_2h"} {
					Expect(kospi).To(HaveKeyWithValue(name, BeNil()))
				}
				Expect(kospi["change_pct"]).To(Equal(json.Number("1")))
				flow := kospi["flow"].(map[string]any)
				Expect(flow["foreign"]).To(Equal(json.Number("0")))
				Expect(flow["etc_foreign"]).To(BeNil())
				Expect(flow["last_ts"]).To(BeNil())
				Expect(result.Envelope.Report["basis_delta_1h"]).To(BeNil())
				Expect(result.Envelope.Report["basis_delta_2h"]).To(BeNil())
				Expect(result.Pulse.KOSPI.Index.LastTS).To(Equal(result.Pulse.Now.Add(-time.Minute)))
				Expect(result.Markdown).To(ContainSubstring("현재 관측 전용"))
				Expect(result.Markdown).NotTo(ContainSubstring("1h"))
				Expect(result.Markdown).NotTo(ContainSubstring("2h"))
				Expect(result.Markdown).NotTo(ContainSubstring("jsonl"))
				for _, artifact := range result.Envelope.Artifacts {
					Expect(artifact.Kind).NotTo(Equal("history"))
					Expect(filepath.Dir(artifact.Path)).To(Equal(output))
					raw, err := os.ReadFile(artifact.Path)
					Expect(err).NotTo(HaveOccurred())
					if artifact.Kind == "json" {
						expected, err := json.Marshal(result.Envelope.Report)
						Expect(err).NotTo(HaveOccurred())
						Expect(raw).To(MatchJSON(expected))
					} else {
						Expect(string(raw)).To(Equal(result.Markdown))
					}
				}
			}
			Expect(calls).To(Equal(2))
			after, err := os.ReadFile(historyPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(before))
			after, err = os.ReadFile(cliMD)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(after)).To(Equal("existing CLI report"))
			observations, err := filepath.Glob(filepath.Join(output, "*.jsonl"))
			Expect(err).NotTo(HaveOccurred())
			Expect(observations).To(BeEmpty())
			reports, err := filepath.Glob(filepath.Join(output, "pulse_current.*.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(reports).To(HaveLen(2))
		})
		It("honors no-save without re-enabling comparisons", func() {
			service := NewWithCollectors(config, Collectors{Pulse: func(context.Context, pulse.Options) (*pulse.Pulse, error) {
				return comparisonPulseFixture(now, 2500), nil
			}})
			result, err := service.RunPulse(context.Background(), pulse.Options{CurrentOnly: true, NoSave: true, StoreDir: output})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Envelope.Artifacts).To(BeEmpty())
			Expect(result.Pulse.Saved).To(BeFalse())
			Expect(result.Pulse.StoredCount).To(BeZero())
			_, err = os.Stat(output)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})
})
