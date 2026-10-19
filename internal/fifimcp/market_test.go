package fifimcp

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/marketreport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeRunner struct {
	premarket func(context.Context, premarket.Options) (*marketreport.Result, error)
	pulse     func(context.Context, pulse.Options) (*marketreport.Result, error)
	snapshot  func(context.Context, snapshot.Options, bool) (*marketreport.Result, error)
}

func (f fakeRunner) RunPremarket(ctx context.Context, o premarket.Options) (*marketreport.Result, error) {
	defer GinkgoRecover()
	return f.premarket(ctx, o)
}
func (f fakeRunner) RunPulse(ctx context.Context, o pulse.Options) (*marketreport.Result, error) {
	defer GinkgoRecover()
	return f.pulse(ctx, o)
}
func (f fakeRunner) RunSnapshot(ctx context.Context, o snapshot.Options, save bool) (*marketreport.Result, error) {
	defer GinkgoRecover()
	return f.snapshot(ctx, o, save)
}

var _ = Describe("Market MCP protocol", func() {
	Context("client interactions", func() {
		var ctx context.Context
		var session *mcp.ClientSession
		var runner fakeRunner
		result := func(kind string) *marketreport.Result {
			return &marketreport.Result{Envelope: marketreport.Envelope{SchemaVersion: 1, ReportType: kind, Report: map[string]any{"missing_value": nil, "status": "TRIGGER_HISTORY_UNKNOWN", "source": "fixture"}, Artifacts: []marketreport.Artifact{}, Warnings: []string{}}}
		}
		BeforeEach(func() {
			GinkgoT().Setenv("MCP_PULSE_OUTPUT_DIR", "")
			GinkgoT().Setenv("PULSE_OUTPUT_DIR", filepath.Join(GinkgoT().TempDir(), "shared-observations"))
			ctx = context.Background()
			runner = fakeRunner{premarket: func(context.Context, premarket.Options) (*marketreport.Result, error) {
				return result("premarket"), nil
			}, pulse: func(context.Context, pulse.Options) (*marketreport.Result, error) {
				return result("intraday-pulse"), nil
			}, snapshot: func(context.Context, snapshot.Options, bool) (*marketreport.Result, error) {
				return result("market-snapshot"), nil
			}}
		})
		connect := func() {
			server := mcp.NewServer(&mcp.Implementation{Name: "market-fixture", Version: "1"}, nil)
			RegisterMarket(server, runner)
			st, ct := mcp.NewInMemoryTransports()
			ss, err := server.Connect(ctx, st, nil)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(ss.Close)
			client := mcp.NewClient(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
			session, err = client.Connect(ctx, ct, nil)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(session.Close)
		}
		It("negotiates and lists exactly three typed market tools", func() {
			connect()
			list, err := session.ListTools(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Tools).To(HaveLen(3))
			var names []string
			for _, tool := range list.Tools {
				names = append(names, tool.Name)
				Expect(tool.OutputSchema).NotTo(BeNil())
				Expect(tool.Annotations.ReadOnlyHint).To(BeFalse())
				Expect(*tool.Annotations.DestructiveHint).To(BeFalse())
				if tool.Name == "market_intraday_pulse" {
					Expect(tool.Description).To(ContainSubstring("without reading or appending observation history or returning interval deltas"))
					Expect(tool.Description).To(ContainSubstring("Saves standalone JSON/Markdown reports and data caches"))
					schema, err := json.Marshal(tool.InputSchema)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(schema)).To(ContainSubstring("Deprecated compatibility input; ignored"))
				}
			}
			Expect(names).To(ConsistOf("market_premarket_report", "market_intraday_pulse", "market_daily_snapshot"))
		})
		It("returns identical structured and JSON text content without dropping missingness", func() {
			connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "market_premarket_report", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			raw, err := json.Marshal(response.StructuredContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Content).To(HaveLen(1))
			Expect(response.Content[0].(*mcp.TextContent).Text).To(MatchJSON(raw))
			var envelope marketreport.Envelope
			Expect(json.Unmarshal(raw, &envelope)).To(Succeed())
			Expect(envelope.Report).To(HaveKeyWithValue("missing_value", BeNil()))
			Expect(envelope.Report["status"]).To(Equal("TRIGGER_HISTORY_UNKNOWN"))
		})
		It("keeps exact source integers in the serialized market report", func() {
			runner.premarket = func(context.Context, premarket.Options) (*marketreport.Result, error) {
				r := result("premarket")
				r.Envelope.Report["amount"] = json.Number("9007199254740993")
				return r, nil
			}
			connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "market_premarket_report", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			Expect(response.Content[0].(*mcp.TextContent).Text).To(ContainSubstring(`"amount":9007199254740993`))
		})
		It("saves current pulse reports separately and keeps dated snapshot saving unchanged", func() {
			runner.pulse = func(_ context.Context, o pulse.Options) (*marketreport.Result, error) {
				Expect(o.NoSave).To(BeFalse())
				Expect(o.CurrentOnly).To(BeTrue())
				Expect(o.StoreDir).To(Equal(".cache/mcp/pulse"))
				return result("intraday-pulse"), nil
			}
			runner.snapshot = func(_ context.Context, o snapshot.Options, save bool) (*marketreport.Result, error) {
				Expect(o.Date).To(Equal("20261015"))
				Expect(save).To(BeTrue())
				return result("market-snapshot"), nil
			}
			connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "market_intraday_pulse", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			response, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "market_daily_snapshot", Arguments: map[string]any{"date": "20261015"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
		})
		DescribeTable("ignores the deprecated nullable history input and always collects current evidence", func(args map[string]any) {
			runner.pulse = func(_ context.Context, options pulse.Options) (*marketreport.Result, error) {
				Expect(options.CurrentOnly).To(BeTrue())
				Expect(options.NoSave).To(BeFalse())
				Expect(options.StoreDir).To(Equal(".cache/mcp/pulse"))
				return result("intraday-pulse"), nil
			}
			connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "market_intraday_pulse", Arguments: args})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
		},
			Entry("omitted", map[string]any{}),
			Entry("null", map[string]any{"use_history": nil}),
			Entry("true", map[string]any{"use_history": true}),
			Entry("false", map[string]any{"use_history": false}),
		)
		It("uses the dedicated MCP pulse directory override without changing other collectors", func() {
			dir := filepath.Join(GinkgoT().TempDir(), "mcp-current")
			GinkgoT().Setenv("MCP_PULSE_OUTPUT_DIR", "  "+dir+"  ")
			runner.pulse = func(_ context.Context, options pulse.Options) (*marketreport.Result, error) {
				Expect(options.CurrentOnly).To(BeTrue())
				Expect(options.StoreDir).To(Equal(dir))
				Expect(options.NoSave).To(BeFalse())
				return result("intraday-pulse"), nil
			}
			runner.premarket = func(_ context.Context, options premarket.Options) (*marketreport.Result, error) {
				Expect(options).To(Equal(premarket.Options{}))
				return result("premarket"), nil
			}
			runner.snapshot = func(_ context.Context, options snapshot.Options, save bool) (*marketreport.Result, error) {
				Expect(options).To(Equal(snapshot.Options{Date: "20261015"}))
				Expect(save).To(BeTrue())
				return result("market-snapshot"), nil
			}
			connect()
			for _, call := range []mcp.CallToolParams{
				{Name: "market_intraday_pulse", Arguments: map[string]any{"use_history": true}},
				{Name: "market_premarket_report", Arguments: map[string]any{}},
				{Name: "market_daily_snapshot", Arguments: map[string]any{"date": "20261015"}},
			} {
				response, err := session.CallTool(ctx, &call)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.IsError).To(BeFalse())
			}
		})
		It("preserves null artifact and warning arrays in a valid report", func() {
			runner.premarket = func(context.Context, premarket.Options) (*marketreport.Result, error) {
				r := result("premarket")
				r.Envelope.Artifacts = nil
				r.Envelope.Warnings = nil
				return r, nil
			}
			connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "market_premarket_report", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			raw, err := json.Marshal(response.StructuredContent)
			Expect(err).NotTo(HaveOccurred())
			var envelope map[string]any
			Expect(json.Unmarshal(raw, &envelope)).To(Succeed())
			Expect(envelope).To(HaveKeyWithValue("artifacts", BeNil()))
			Expect(envelope).To(HaveKeyWithValue("warnings", BeNil()))
		})
		It("returns total failure as a tool error with inspectable diagnostic JSON", func() {
			runner.premarket = func(context.Context, premarket.Options) (*marketreport.Result, error) {
				r := result("premarket")
				r.Envelope.Warnings = []string{"upstream unavailable"}
				return r, errors.New("upstream unavailable")
			}
			connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "market_premarket_report", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeTrue())
			Expect(response.StructuredContent).NotTo(BeNil())
		})
		It("rejects unknown tool arguments before invoking a collector", func() {
			runner.premarket = func(context.Context, premarket.Options) (*marketreport.Result, error) {
				Fail("collector invoked with invalid arguments")
				return nil, nil
			}
			connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "market_premarket_report", Arguments: map[string]any{"command": "arbitrary-shell"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeTrue())
		})
		It("propagates MCP caller cancellation to ongoing collection", func() {
			started, stopped := make(chan struct{}), make(chan struct{})
			runner.premarket = func(ctx context.Context, _ premarket.Options) (*marketreport.Result, error) {
				close(started)
				<-ctx.Done()
				close(stopped)
				return result("premarket"), ctx.Err()
			}
			connect()
			callCtx, cancel := context.WithCancel(ctx)
			done := make(chan error, 1)
			go func() {
				_, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: "market_premarket_report", Arguments: map[string]any{}})
				done <- err
			}()
			Eventually(started, time.Second).Should(BeClosed())
			cancel()
			Eventually(done, time.Second).Should(Receive(HaveOccurred()))
			Eventually(stopped, time.Second).Should(BeClosed())
		})
	})
})
