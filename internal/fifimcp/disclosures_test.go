package fifimcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Disclosure MCP", func() {
	Context("client interactions", func() {
		connect := func(handler http.HandlerFunc) *mcp.ClientSession {
			httpClient := &http.Client{Transport: disclosureRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				defer GinkgoRecover()
				recorder := httptest.NewRecorder()
				handler(recorder, request)
				if err := request.Context().Err(); err != nil {
					return nil, err
				}
				return recorder.Result(), nil
			})}
			server := mcp.NewServer(&mcp.Implementation{Name: "disclosure-fixture", Version: "1"}, nil)
			RegisterDisclosures(server, "https://fixture.invalid/api/v1", httpClient)
			st, ct := mcp.NewInMemoryTransports()
			ctx := context.Background()
			ss, err := server.Connect(ctx, st, nil)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(ss.Close)
			client := mcp.NewClient(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
			cs, err := client.Connect(ctx, ct, nil)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(cs.Close)
			return cs
		}
		It("preserves the three established disclosure tool names and schemas", func() {
			cs := connect(func(http.ResponseWriter, *http.Request) { Fail("list must not fetch upstream") })
			tools, err := cs.ListTools(context.Background(), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(tools.Tools).To(HaveLen(3))
			var names []string
			for _, tool := range tools.Tools {
				names = append(names, tool.Name)
				Expect(tool.OutputSchema).NotTo(BeNil())
				raw, err := json.Marshal(tool.OutputSchema)
				Expect(err).NotTo(HaveOccurred())
				var schema map[string]any
				Expect(json.Unmarshal(raw, &schema)).To(Succeed())
				Expect(schema).To(HaveKeyWithValue("additionalProperties", true))
				Expect(tool.Annotations.ReadOnlyHint).To(BeTrue())
			}
			Expect(names).To(ConsistOf("disclosures_reports_by_corp_name", "reports_summary_all_companies", "reports_summary_and_raw_by_receipt_number"))
		})
		It("preserves arbitrary API object values including arrays, scalars and null", func() {
			body := `{"reports":[],"count":0,"has_more":false,"next":null,"label":"기업","metadata":{"source":"fixture"}}`
			cs := connect(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) })
			response, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "reports_summary_all_companies", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			Expect(response.Content[0].(*mcp.TextContent).Text).To(MatchJSON(body))
			raw, err := json.Marshal(response.StructuredContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(raw)).To(MatchJSON(body))
		})
		It("encodes company filters and returns matching structured/text JSON", func() {
			requests := make(chan *http.Request, 1)
			cs := connect(func(w http.ResponseWriter, r *http.Request) {
				requests <- r
				fmt.Fprint(w, `{"reports":[{"name":"삼성전자"}]}`)
			})
			response, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "disclosures_reports_by_corp_name", Arguments: map[string]any{"corp_name": "삼성 & 전자", "limit": 5}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			var request *http.Request
			Expect(requests).To(Receive(&request))
			Expect(request.URL.Path).To(Equal("/api/v1/mcp/reports/by-corp-name"))
			Expect(request.URL.Query().Get("corp_name")).To(Equal("삼성 & 전자"))
			Expect(request.URL.Query().Get("limit")).To(Equal("5"))
			raw, err := json.Marshal(response.StructuredContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Content[0].(*mcp.TextContent).Text).To(MatchJSON(raw))
			Expect(response.Content[0].(*mcp.TextContent).Text).To(MatchJSON(`{"reports":[{"name":"삼성전자"}]}`))
		})
		DescribeTable("preserves REST paths, filters and default limits", func(name string, args map[string]any, path string, query map[string]string) {
			requests := make(chan *http.Request, 1)
			cs := connect(func(w http.ResponseWriter, r *http.Request) {
				requests <- r
				fmt.Fprint(w, `{"reports":[],"missing":null}`)
			})
			response, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			var request *http.Request
			Expect(requests).To(Receive(&request))
			Expect(request.Method).To(Equal(http.MethodGet))
			Expect(request.URL.EscapedPath()).To(Equal(path))
			Expect(request.URL.Query()).To(HaveLen(len(query)))
			for key, value := range query {
				Expect(request.URL.Query().Get(key)).To(Equal(value))
			}
			Expect(response.Content[0].(*mcp.TextContent).Text).To(MatchJSON(`{"reports":[],"missing":null}`))
		},
			Entry("company-name default", "disclosures_reports_by_corp_name", map[string]any{"corp_name": " 삼성 "}, "/api/v1/mcp/reports/by-corp-name", map[string]string{"corp_name": "삼성", "limit": "10"}),
			Entry("all-company default", "reports_summary_all_companies", map[string]any{}, "/api/v1/reports", map[string]string{"limit": "10"}),
			Entry("summary filters", "reports_summary_all_companies", map[string]any{"corp_code": " 00126380 ", "limit": 20, "start_date": "2026-10-01", "end_date": "2026-10-17"}, "/api/v1/reports", map[string]string{"corp_code": "00126380", "limit": "20", "start_date": "2026-10-01", "end_date": "2026-10-17"}),
			Entry("receipt", "reports_summary_and_raw_by_receipt_number", map[string]any{"receipt_number": " 20261016001234 "}, "/api/v1/reports/receipt/20261016001234", map[string]string{}),
			Entry("escaped receipt", "reports_summary_and_raw_by_receipt_number", map[string]any{"receipt_number": "receipt/one"}, "/api/v1/reports/receipt/receipt%2Fone", map[string]string{}),
		)
		DescribeTable("validates inputs before upstream requests", func(name string, args map[string]any) {
			cs := connect(func(http.ResponseWriter, *http.Request) { Fail("invalid tool input reached upstream") })
			response, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeTrue())
		}, Entry("empty receipt", "reports_summary_and_raw_by_receipt_number", map[string]any{"receipt_number": " "}), Entry("large limit", "reports_summary_all_companies", map[string]any{"limit": 501}), Entry("fractional limit", "reports_summary_all_companies", map[string]any{"limit": 1.5}), Entry("bad date", "reports_summary_all_companies", map[string]any{"start_date": "yesterday"}), Entry("empty company", "disclosures_reports_by_corp_name", map[string]any{"corp_name": " "}), Entry("company limit", "disclosures_reports_by_corp_name", map[string]any{"corp_name": "삼성", "limit": 101}), Entry("negative limit", "reports_summary_all_companies", map[string]any{"limit": -1}), Entry("reversed range", "reports_summary_all_companies", map[string]any{"start_date": "2026-10-17", "end_date": "2026-10-01"}))
		It("returns upstream business failure as a tool execution error", func() {
			cs := connect(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
			response, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "reports_summary_all_companies", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeTrue())
		})
		It("leaves the session usable after an upstream failure", func() {
			var calls atomic.Int32
			cs := connect(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				fmt.Fprint(w, `{"reports":[]}`)
			})
			first, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "reports_summary_all_companies", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(first.IsError).To(BeTrue())
			next, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "disclosures_reports_by_corp_name", Arguments: map[string]any{"corp_name": "삼성"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(next.IsError).To(BeFalse())
			Expect(next.Content[0].(*mcp.TextContent).Text).To(MatchJSON(`{"reports":[]}`))
		})
		It("preserves large JSON integers and null fields on the wire", func() {
			cs := connect(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `{"amount":9007199254740993,"missing":null}`)
			})
			response, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "reports_summary_all_companies", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeFalse())
			text := response.Content[0].(*mcp.TextContent).Text
			Expect(text).To(ContainSubstring(`"amount":9007199254740993`))
			Expect(text).To(ContainSubstring(`"missing":null`))
		})
		DescribeTable("rejects malformed or oversized upstream JSON", func(body string) {
			cs := connect(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) })
			response, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "reports_summary_all_companies", Arguments: map[string]any{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.IsError).To(BeTrue())
		}, Entry("not JSON", "invalid"), Entry("null object", "null"), Entry("array instead of object", "[]"), Entry("body limit", `{"reports":"`+strings.Repeat("x", 8<<20)+`"}`))
		It("cancels upstream HTTP when the LLM client cancels", func() {
			started, stopped := make(chan struct{}), make(chan struct{})
			cs := connect(func(_ http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(stopped) })
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				_, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "reports_summary_all_companies", Arguments: map[string]any{}})
				done <- err
			}()
			Eventually(started, time.Second).Should(BeClosed())
			cancel()
			Eventually(done, time.Second).Should(Receive(HaveOccurred()))
			Eventually(stopped, time.Second).Should(BeClosed())
		})
	})
})

type disclosureRoundTripFunc func(*http.Request) (*http.Response, error)

func (f disclosureRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
