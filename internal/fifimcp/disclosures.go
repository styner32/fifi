// The disclosure MCP adapter keeps existing tool names and HTTP API routes while
// delegating protocol negotiation, validation and cancellation to the MCP SDK.
package fifimcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type reportsByNameArgs struct {
	CorpName string `json:"corp_name" jsonschema:"Partial or full company name (case-insensitive)."`
	Limit    int    `json:"limit,omitempty" jsonschema:"Maximum reports, 1 to 100; default 10."`
}
type summariesArgs struct {
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum reports, 1 to 500; default 10."`
	CorpCode  string `json:"corp_code,omitempty"`
	StartDate string `json:"start_date,omitempty" jsonschema:"Inclusive receipt date YYYY-MM-DD."`
	EndDate   string `json:"end_date,omitempty" jsonschema:"Exclusive receipt date YYYY-MM-DD."`
}
type receiptArgs struct {
	ReceiptNumber string `json:"receipt_number" jsonschema:"Nonempty DART receipt number."`
}

// Preserve the upstream JSON object shape used by existing MCP consumers.
type disclosureResult map[string]any

type disclosureAPI struct {
	baseURL string
	client  *http.Client
}

// RegisterDisclosures adds the established disclosure tools to an existing MCP
// server. Registration performs no upstream requests or database/LLM work.
func RegisterDisclosures(server *mcp.Server, baseURL string, client *http.Client) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	api := &disclosureAPI{baseURL: strings.TrimRight(baseURL, "/"), client: client}
	outputSchema := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	annotations := func() *mcp.ToolAnnotations {
		yes := true
		return &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &yes}
	}
	mcp.AddTool(server, &mcp.Tool{Name: "disclosures_reports_by_corp_name", Description: "List recent reports or disclosures matching a partial corp_name.", Annotations: annotations(), OutputSchema: outputSchema}, func(ctx context.Context, _ *mcp.CallToolRequest, args reportsByNameArgs) (*mcp.CallToolResult, any, error) {
		name := strings.TrimSpace(args.CorpName)
		if name == "" {
			return nil, nil, fmt.Errorf("corp_name must be nonempty")
		}
		limit, err := validatedLimit(args.Limit, 100)
		if err != nil {
			return nil, nil, err
		}
		return api.get(ctx, "/mcp/reports/by-corp-name", url.Values{"corp_name": {name}, "limit": {strconv.Itoa(limit)}})
	})
	mcp.AddTool(server, &mcp.Tool{Name: "reports_summary_all_companies", Description: "Get report summaries across all companies.", Annotations: annotations(), OutputSchema: outputSchema}, func(ctx context.Context, _ *mcp.CallToolRequest, args summariesArgs) (*mcp.CallToolResult, any, error) {
		limit, err := validatedLimit(args.Limit, 500)
		if err != nil {
			return nil, nil, err
		}
		values := url.Values{"limit": {strconv.Itoa(limit)}}
		if code := strings.TrimSpace(args.CorpCode); code != "" {
			values.Set("corp_code", code)
		}
		for _, date := range []struct{ key, value string }{{"start_date", args.StartDate}, {"end_date", args.EndDate}} {
			if date.value != "" {
				if _, err := time.Parse("2006-01-02", date.value); err != nil {
					return nil, nil, fmt.Errorf("%s must use YYYY-MM-DD", date.key)
				}
				values.Set(date.key, date.value)
			}
		}
		if args.StartDate != "" && args.EndDate != "" && args.EndDate <= args.StartDate {
			return nil, nil, fmt.Errorf("end_date must be after start_date")
		}
		return api.get(ctx, "/reports", values)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "reports_summary_and_raw_by_receipt_number", Description: "Get report summary and raw report by receipt number.", Annotations: annotations(), OutputSchema: outputSchema}, func(ctx context.Context, _ *mcp.CallToolRequest, args receiptArgs) (*mcp.CallToolResult, any, error) {
		number := strings.TrimSpace(args.ReceiptNumber)
		if number == "" {
			return nil, nil, fmt.Errorf("receipt_number must be nonempty")
		}
		return api.get(ctx, "/reports/receipt/"+url.PathEscape(number), nil)
	})
}

func validatedLimit(limit, maximum int) (int, error) {
	if limit == 0 {
		return 10, nil
	}
	if limit < 1 || limit > maximum {
		return 0, fmt.Errorf("limit must be between 1 and %d", maximum)
	}
	return limit, nil
}

func (api *disclosureAPI) get(ctx context.Context, path string, query url.Values) (*mcp.CallToolResult, any, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	endpoint := api.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid disclosure API URL")
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("disclosure API request failed: %w", ctxError(ctx, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("disclosure API returned HTTP %d", resp.StatusCode)
	}
	const maxBody = 8 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read disclosure API response")
	}
	if len(raw) > maxBody {
		return nil, nil, fmt.Errorf("disclosure API response exceeds 8 MiB")
	}
	if !json.Valid(raw) {
		return nil, nil, fmt.Errorf("disclosure API returned invalid JSON")
	}
	var data disclosureResult
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&data); err != nil {
		return nil, nil, fmt.Errorf("disclosure API returned invalid JSON")
	}
	if data == nil {
		return nil, nil, fmt.Errorf("disclosure API returned no JSON object")
	}
	// Typed AddTool output normalization round-trips through float64 even for
	// an unconstrained object schema. Keep SDK input validation, but return a nil
	// typed output and explicit raw content so provider integers stay exact.
	return &mcp.CallToolResult{
		StructuredContent: json.RawMessage(raw),
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
	}, nil, nil
}
func ctxError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
