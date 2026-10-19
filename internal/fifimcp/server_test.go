package fifimcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/marketreport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type combinedMarketCall struct {
	kind              string
	noSave, noHistory bool
	date              string
	save              bool
}

type combinedRunner struct {
	calls        chan combinedMarketCall
	premarketErr error
}

func (runner *combinedRunner) RunPremarket(_ context.Context, options premarket.Options) (*marketreport.Result, error) {
	runner.calls <- combinedMarketCall{kind: marketreport.Premarket, noSave: options.NoSave}
	result := combinedMarketResult(marketreport.Premarket)
	if runner.premarketErr != nil {
		result.Envelope.Warnings = []string{runner.premarketErr.Error()}
	}
	return result, runner.premarketErr
}

func (runner *combinedRunner) RunPulse(_ context.Context, options pulse.Options) (*marketreport.Result, error) {
	runner.calls <- combinedMarketCall{kind: marketreport.IntradayPulse, noSave: options.NoSave, noHistory: options.NoHistory}
	return combinedMarketResult(marketreport.IntradayPulse), nil
}

func (runner *combinedRunner) RunSnapshot(_ context.Context, options snapshot.Options, save bool) (*marketreport.Result, error) {
	runner.calls <- combinedMarketCall{kind: marketreport.DailySnapshot, date: options.Date, save: save}
	return combinedMarketResult(marketreport.DailySnapshot), nil
}

func combinedMarketResult(kind string) *marketreport.Result {
	return &marketreport.Result{Envelope: marketreport.Envelope{
		SchemaVersion: 1, ReportType: kind,
		Report:    map[string]any{"missing_value": nil, "observed_zero": 0, "status": "TRIGGER_HISTORY_UNKNOWN", "source": "fixture"},
		Artifacts: []marketreport.Artifact{}, Warnings: []string{},
	}}
}

type combinedTransport func(*http.Request) (*http.Response, error)

func (transport combinedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	defer GinkgoRecover()
	return transport(request)
}

type combinedHTTPRequest struct {
	method, host, path string
	query              url.Values
}

func combinedJSON(response *mcp.CallToolResult) map[string]any {
	GinkgoHelper()
	Expect(response.StructuredContent).NotTo(BeNil())
	raw, err := json.Marshal(response.StructuredContent)
	Expect(err).NotTo(HaveOccurred())
	Expect(response.Content).To(HaveLen(1))
	text, ok := response.Content[0].(*mcp.TextContent)
	Expect(ok).To(BeTrue())
	Expect(text.Text).To(MatchJSON(raw))
	var data map[string]any
	Expect(json.Unmarshal(raw, &data)).To(Succeed())
	return data
}

var _ = Describe("Unified Fifi MCP session", func() {
	var ctx context.Context
	var runner *combinedRunner
	var transport combinedTransport
	var requests chan combinedHTTPRequest

	BeforeEach(func() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		DeferCleanup(cancel)
		runner = &combinedRunner{calls: make(chan combinedMarketCall, 8)}
		requests = make(chan combinedHTTPRequest, 8)
		transport = func(request *http.Request) (*http.Response, error) {
			requests <- combinedHTTPRequest{method: request.Method, host: request.URL.Host, path: request.URL.Path, query: request.URL.Query()}
			raw, err := json.Marshal(map[string]any{"fixture_route": request.URL.Path, "missing_value": nil, "observed_zero": 0, "report_name": "공시 원문 fixture"})
			Expect(err).NotTo(HaveOccurred())
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(raw))), Request: request}, nil
		}
	})

	connect := func() *mcp.ClientSession {
		GinkgoHelper()
		server := NewServer(Options{Market: runner, DisclosureBaseURL: "https://disclosure.fixture.invalid/api/v1/", HTTPClient: &http.Client{Transport: transport}})
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		serverSession, err := server.Connect(ctx, serverTransport, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(serverSession.Close)
		client := mcp.NewClient(&mcp.Implementation{Name: "combined-fixture", Version: "1"}, nil)
		session, err := client.Connect(ctx, clientTransport, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(session.Close)
		return session
	}

	Context("when discovering the common server", func() {
		It("identifies itself as fifi and advertises exactly six unique typed tools with accurate read-only hints", func() {
			session := connect()
			Expect(session.InitializeResult().ServerInfo.Name).To(Equal("fifi"))
			list, err := session.ListTools(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Tools).To(HaveLen(6))
			var names []string
			for _, tool := range list.Tools {
				names = append(names, tool.Name)
				Expect(tool.Annotations).NotTo(BeNil())
				Expect(tool.Annotations.ReadOnlyHint).To(Equal(!strings.HasPrefix(tool.Name, "market_")))
				for _, schema := range []any{tool.InputSchema, tool.OutputSchema} {
					Expect(schema).NotTo(BeNil())
					raw, err := json.Marshal(schema)
					Expect(err).NotTo(HaveOccurred())
					var object map[string]any
					Expect(json.Unmarshal(raw, &object)).To(Succeed())
					Expect(object).To(HaveKeyWithValue("type", "object"))
				}
			}
			Expect(names).To(ConsistOf("market_premarket_report", "market_intraday_pulse", "market_daily_snapshot", "disclosures_reports_by_corp_name", "reports_summary_all_companies", "reports_summary_and_raw_by_receipt_number"))
			Expect(runner.calls).To(BeEmpty())
			Expect(requests).To(BeEmpty())
		})
	})

	Context("when market and disclosure tools share one session", func() {
		It("routes all six calls independently and preserves structured and text JSON, including null and zero", func() {
			session := connect()
			calls := []mcp.CallToolParams{
				{Name: "market_premarket_report", Arguments: map[string]any{}},
				{Name: "disclosures_reports_by_corp_name", Arguments: map[string]any{"corp_name": "삼성 & 전자", "limit": 5}},
				{Name: "market_intraday_pulse", Arguments: map[string]any{"use_history": false}},
				{Name: "reports_summary_all_companies", Arguments: map[string]any{"corp_code": "00126380", "start_date": "2026-10-01", "end_date": "2026-10-16", "limit": 7}},
				{Name: "market_daily_snapshot", Arguments: map[string]any{"date": "20261015"}},
				{Name: "reports_summary_and_raw_by_receipt_number", Arguments: map[string]any{"receipt_number": "20261015000123"}},
			}
			marketTypes := map[string]string{"market_premarket_report": marketreport.Premarket, "market_intraday_pulse": marketreport.IntradayPulse, "market_daily_snapshot": marketreport.DailySnapshot}
			disclosureRoutes := map[string]string{"disclosures_reports_by_corp_name": "/api/v1/mcp/reports/by-corp-name", "reports_summary_all_companies": "/api/v1/reports", "reports_summary_and_raw_by_receipt_number": "/api/v1/reports/receipt/20261015000123"}
			for _, call := range calls {
				response, err := session.CallTool(ctx, &call)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.IsError).To(BeFalse())
				data := combinedJSON(response)
				if kind, market := marketTypes[call.Name]; market {
					Expect(data).To(HaveKeyWithValue("report_type", kind))
					Expect(data["report"]).To(SatisfyAll(HaveKeyWithValue("missing_value", BeNil()), HaveKeyWithValue("observed_zero", float64(0)), HaveKeyWithValue("status", "TRIGGER_HISTORY_UNKNOWN")))
				} else {
					Expect(data).To(SatisfyAll(HaveKeyWithValue("fixture_route", disclosureRoutes[call.Name]), HaveKeyWithValue("missing_value", BeNil()), HaveKeyWithValue("observed_zero", float64(0))))
					Expect(data).NotTo(HaveKey("schema_version"))
				}
			}
			var marketCalls []combinedMarketCall
			for range 3 {
				var call combinedMarketCall
				Expect(runner.calls).To(Receive(&call))
				marketCalls = append(marketCalls, call)
			}
			Expect(marketCalls).To(Equal([]combinedMarketCall{{kind: marketreport.Premarket}, {kind: marketreport.IntradayPulse, noHistory: true}, {kind: marketreport.DailySnapshot, date: "20261015", save: true}}))
			var httpCalls []combinedHTTPRequest
			for range 3 {
				var request combinedHTTPRequest
				Expect(requests).To(Receive(&request))
				httpCalls = append(httpCalls, request)
			}
			Expect(httpCalls).To(Equal([]combinedHTTPRequest{
				{method: http.MethodGet, host: "disclosure.fixture.invalid", path: "/api/v1/mcp/reports/by-corp-name", query: url.Values{"corp_name": {"삼성 & 전자"}, "limit": {"5"}}},
				{method: http.MethodGet, host: "disclosure.fixture.invalid", path: "/api/v1/reports", query: url.Values{"corp_code": {"00126380"}, "start_date": {"2026-10-01"}, "end_date": {"2026-10-16"}, "limit": {"7"}}},
				{method: http.MethodGet, host: "disclosure.fixture.invalid", path: "/api/v1/reports/receipt/20261015000123", query: url.Values{}},
			}))
			Expect(runner.calls).To(BeEmpty())
			Expect(requests).To(BeEmpty())
		})

		It("keeps market calls and discovery available after a disclosure API failure", func() {
			transport = func(*http.Request) (*http.Response, error) {
				return nil, errors.New("fixture disclosure API unavailable")
			}
			session := connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "reports_summary_all_companies", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeTrue())
			response, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "market_intraday_pulse", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			Expect(combinedJSON(response)).To(HaveKeyWithValue("report_type", marketreport.IntradayPulse))
			list, err := session.ListTools(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Tools).To(HaveLen(6))
		})

		It("keeps disclosure calls available after a market collection failure", func() {
			runner.premarketErr = errors.New("fixture market unavailable")
			session := connect()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "market_premarket_report", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeTrue())
			Expect(combinedJSON(response)).To(HaveKeyWithValue("warnings", ContainElement("fixture market unavailable")))
			response, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "disclosures_reports_by_corp_name", Arguments: map[string]any{"corp_name": "fixture"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			Expect(combinedJSON(response)).To(HaveKeyWithValue("fixture_route", "/api/v1/mcp/reports/by-corp-name"))
			list, err := session.ListTools(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Tools).To(HaveLen(6))
		})
	})
})
