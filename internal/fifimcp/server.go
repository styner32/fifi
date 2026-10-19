// Package fifimcp exposes market reports and stored disclosures in one MCP session.
package fifimcp

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fifi/internal/marketreport"
	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options inject collectors and the disclosure HTTP adapter without starting them.
type Options struct {
	Market            MarketRunner
	DisclosureBaseURL string
	HTTPClient        *http.Client
}

// NewServer registers all six existing tools on a single protocol server.
// Upstream services are contacted only when their corresponding tool is called.
func NewServer(options Options) *mcp.Server {
	if options.Market == nil {
		options.Market = marketreport.New(marketreport.ConfigFromEnv())
	}
	baseURL := strings.TrimSpace(options.DisclosureBaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("DART_FILING_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("KOSIS_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080/api/v1"
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fifi", Version: "1.0.0"}, nil)
	RegisterMarket(server, options.Market)
	RegisterDisclosures(server, baseURL, options.HTTPClient)
	return server
}

// RunStdio is shared by the canonical command and compatibility entry points.
func RunStdio() error {
	log.SetOutput(os.Stderr)
	_ = godotenv.Load()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return NewServer(Options{}).Run(ctx, &mcp.StdioTransport{})
}
