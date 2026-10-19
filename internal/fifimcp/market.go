package fifimcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/marketreport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MarketRunner is the report service shared with the market CLI.
type MarketRunner interface {
	RunPremarket(context.Context, premarket.Options) (*marketreport.Result, error)
	RunPulse(context.Context, pulse.Options) (*marketreport.Result, error)
	RunSnapshot(context.Context, snapshot.Options, bool) (*marketreport.Result, error)
}

type emptyArgs struct{}
type pulseArgs struct {
	UseHistory *bool `json:"use_history,omitempty" jsonschema:"Deprecated compatibility input; ignored. This tool always collects current evidence without reading or appending observation history or returning interval deltas."`
}
type snapshotArgs struct {
	Date string `json:"date,omitempty" jsonschema:"KST business date YYYYMMDD or YYYY-MM-DD; default today. Future dates are rejected."`
}

// RegisterMarket adds the three market report tools to the supplied server.
func RegisterMarket(server *mcp.Server, service MarketRunner) {
	outputSchema := marketOutputSchema()
	annotations := func() *mcp.ToolAnnotations {
		no, yes := false, true
		return &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &no, OpenWorldHint: &yes}
	}
	mcp.AddTool(server, &mcp.Tool{Name: "market_premarket_report", Description: "Collect Korean premarket evidence with source timestamps, cutoff, missing-data statuses and vulnerability board. Saves local reports/history and data caches. No orders, filing sync or LLM API calls.", Annotations: annotations(), OutputSchema: outputSchema}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		return toolResponse(service.RunPremarket(ctx, premarket.Options{}))
	})
	mcp.AddTool(server, &mcp.Tool{Name: "market_intraday_pulse", Description: "Collect current KOSPI/KOSDAQ indices, investor flows, futures, FX and safety-device evidence without reading or appending observation history or returning interval deltas. Retains daily changes and source timestamps. Saves standalone JSON/Markdown reports and data caches. No orders, filing sync or LLM API calls.", Annotations: annotations(), InputSchema: pulseInputSchema(), OutputSchema: outputSchema}, func(ctx context.Context, _ *mcp.CallToolRequest, _ pulseArgs) (*mcp.CallToolResult, any, error) {
		dir := strings.TrimSpace(os.Getenv("MCP_PULSE_OUTPUT_DIR"))
		if dir == "" {
			dir = ".cache/mcp/pulse"
		}
		return toolResponse(service.RunPulse(ctx, pulse.Options{CurrentOnly: true, StoreDir: dir}))
	})
	mcp.AddTool(server, &mcp.Tool{Name: "market_daily_snapshot", Description: "Collect a dated Korean market snapshot. Historical dates exclude current-only endpoints; post-close output remains unreconciled when sources are unverified. Saves market JSON/Markdown and caches. No orders, DART sync or OpenAI calls.", Annotations: annotations(), OutputSchema: outputSchema}, func(ctx context.Context, _ *mcp.CallToolRequest, args snapshotArgs) (*mcp.CallToolResult, any, error) {
		return toolResponse(service.RunSnapshot(ctx, snapshot.Options{Date: args.Date}, true))
	})
}

func toolResponse(result *marketreport.Result, err error) (*mcp.CallToolResult, any, error) {
	if result == nil {
		return nil, nil, err
	}
	raw, marshalErr := json.Marshal(result.Envelope)
	if marshalErr != nil {
		return nil, nil, marshalErr
	}
	// Preserve json.Number values from the shared report serializer. SDK typed
	// output normalization would otherwise round large integers through float64.
	return &mcp.CallToolResult{IsError: err != nil, StructuredContent: json.RawMessage(raw), Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
}
