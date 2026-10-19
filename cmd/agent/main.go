package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/external/naver"
	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/marketreport"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	_ = godotenv.Load()
	if len(args) < 2 || args[0] != "report" {
		return fmt.Errorf("usage: go run ./cmd/agent report <market-snapshot|filings-snapshot|credit-balance|intraday-pulse|premarket|rsi|safety-devices> [args...]")
	}
	switch args[1] {
	case "market-snapshot":
		return runMarketSnapshot(args[2:])
	case "filings-snapshot":
		return runFilingsSnapshot(args[2:])
	case "credit-balance":
		return runCreditBalance(args[2:])
	case "intraday-pulse":
		return runIntradayPulse(args[2:])
	case "premarket":
		return runPremarket(args[2:])
	case "rsi":
		return runRSI(args[2:])
	case "safety-devices":
		return runIntradayPulse(append([]string{"--safety-only"}, args[2:]...))
	default:
		return fmt.Errorf("unknown report type %q (supported: market-snapshot, filings-snapshot, credit-balance, intraday-pulse, premarket, rsi, safety-devices)", args[1])
	}
}

func runMarketSnapshot(args []string) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	config := marketreport.ConfigFromEnv()
	result, err := marketreport.New(config).RunSnapshot(context.Background(), opts, true)
	if result != nil {
		fmt.Print(result.Markdown)
		logReportResult(result)
	}
	if err != nil {
		return err
	}
	// Filing sync remains CLI-only and is intentionally outside marketreport.
	runFilingsSnapshotForMarketSnapshot(result.Snapshot.BusinessDate, config.SnapshotDir)
	return nil
}

func runIntradayPulse(args []string) error {
	fs := flag.NewFlagSet("intraday-pulse", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	storeDir := fs.String("store-dir", envDefault("PULSE_OUTPUT_DIR", ".cache/pulse"), "펄스 적립 디렉터리")
	noSave := fs.Bool("no-save", false, "보고서/이력 저장 생략 (인증 및 데이터 캐시는 갱신 가능)")
	noHistory := fs.Bool("no-history", false, "이전 적립/누적 레코드 무시")
	fs.BoolVar(noHistory, "no-prev", false, "no-history와 동일")
	asJSON := fs.Bool("json", false, "JSON 출력")
	safetyOnly := fs.Bool("safety-only", false, "서킷브레이커 및 사이드카 상태만 렌더링")
	premarketFlag := fs.Bool("premarket", false, "개장전 취약도 보드 실행")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *premarketFlag {
		forward := []string{}
		if *noSave {
			forward = append(forward, "--no-save")
		}
		if *asJSON {
			forward = append(forward, "--json")
		}
		return runPremarket(forward)
	}
	result, err := marketreport.New(marketreport.ConfigFromEnv()).RunPulse(context.Background(), pulse.Options{StoreDir: *storeDir, Lookback: 2 * time.Hour, NoSave: *noSave, NoHistory: *noHistory, JSON: *asJSON})
	if result != nil {
		logReportResult(result)
		if *asJSON {
			var value any = result.Envelope.Report
			if *safetyOnly && result.Pulse != nil {
				value = result.Pulse.Safety
			}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if encodeErr := encoder.Encode(value); encodeErr != nil {
				return encodeErr
			}
		} else if *safetyOnly && result.Pulse != nil {
			fmt.Print(pulse.RenderSafetyOnly(result.Pulse))
		} else {
			fmt.Print(result.Markdown)
		}
	}
	return err
}

func runPremarket(args []string) error {
	fs := flag.NewFlagSet("premarket", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	storeDir := fs.String("store-dir", envDefault("PREMARKET_STORE_DIR", ".cache/premarket"), "개장전 적립 디렉터리")
	noSave := fs.Bool("no-save", false, "보고서/이력 저장 생략 (인증 및 데이터 캐시는 갱신 가능)")
	asJSON := fs.Bool("json", false, "JSON 출력")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := marketreport.New(marketreport.ConfigFromEnv()).RunPremarket(context.Background(), premarket.Options{StoreDir: *storeDir, NoSave: *noSave, JSON: *asJSON})
	if result != nil {
		logReportResult(result)
		if *asJSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if encodeErr := encoder.Encode(result.Envelope.Report); encodeErr != nil {
				return encodeErr
			}
		} else {
			fmt.Print(result.Markdown)
		}
	}
	return err
}

func logReportResult(result *marketreport.Result) {
	for _, warning := range result.Envelope.Warnings {
		fmt.Fprintln(os.Stderr, "[market-report] "+warning)
	}
	for _, artifact := range result.Envelope.Artifacts {
		fmt.Fprintf(os.Stderr, "[market-report] %s saved: %s\n", artifact.Kind, artifact.Path)
	}
}

func newKISClient() (*auth.KIClient, error) {
	appKey, appSecret := strings.TrimSpace(os.Getenv("APP_KEY")), strings.TrimSpace(os.Getenv("APP_SECRET"))
	if appKey == "" || appSecret == "" {
		return nil, fmt.Errorf("APP_KEY and APP_SECRET are required")
	}
	client := auth.NewKIClient(appKey, appSecret, "https://openapi.koreainvestment.com:9443", os.Getenv("USER_AGENT"))
	client.Client = &http.Client{Timeout: 15 * time.Second}
	client.SetTokenCachePath(envDefault("AUTH_TOKEN_FILE", ".auth_token.json"))
	return client, nil
}
func newExternalClients(client *auth.KIClient) (*yahoo.Client, *naver.Client) {
	ua := os.Getenv("USER_AGENT")
	yahooClient := yahoo.NewClient(client.Client, yahoo.Config{UserAgent: ua})
	naverClient := naver.NewClient(client.Client, ua)
	return yahooClient, naverClient
}
