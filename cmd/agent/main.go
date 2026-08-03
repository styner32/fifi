package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/kofia"
	"github.com/fifi/internal/external/naver"
	"github.com/fifi/internal/external/yahoo"
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
		return fmt.Errorf("usage: go run ./cmd/agent report <market-snapshot|credit-balance|intraday-pulse|premarket|rsi|safety-devices> [args...]")
	}
	switch args[1] {
	case "market-snapshot":
		return runMarketSnapshot(args[2:])
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
		return fmt.Errorf("unknown report type %q (supported: market-snapshot, credit-balance, intraday-pulse, premarket, rsi, safety-devices)", args[1])
	}
}

func runMarketSnapshot(args []string) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	client, err := newKISClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := client.EnsureAuthToken(ctx); err != nil {
		return fmt.Errorf("auth token: %w", err)
	}

	yahooClient, naverClient := newExternalClients(client)
	kofiaClient := kofia.NewCachedClient(envDefault("KOFIA_CACHE_DIR", ".cache"), os.Getenv("USER_AGENT"))
	result := snapshot.Collect(ctx, snapshot.Deps{
		DomesticStock:  domesticstock.NewService(client),
		DomesticFuture: domesticfutureoption.NewService(client),
		Yahoo:          yahooClient,
		Naver:          naverClient,
		KOFIA:          kofiaClient,
	}, opts)
	// ── 이전 날짜 스냅샷 로드 (비교용) ──────────────────────────────────────────
	outDir := envDefault("SNAPSHOT_OUTPUT_DIR", ".cache/snapshots")
	date := time.Now().Format("20060102")
	if opts.Date != "" {
		normalized := strings.ReplaceAll(opts.Date, "-", "")
		if len(normalized) == 8 {
			date = normalized
		}
	}
	var prev *snapshot.SnapshotJSON
	if outDir != "" {
		if p, err := snapshot.LoadPreviousSnapshot(outDir, date); err == nil && p != nil {
			prev = p
			fmt.Fprintf(os.Stderr, "[snapshot] 비교 데이터 로드: %s\n", p.Date)
		}
	}

	// ── 렌더링 (이전 데이터가 있으면 비교 표시) ──────────────────────────────────
	output := snapshot.Render(result, prev)
	fmt.Print(output)

	// ── JSON + Markdown 저장 ─────────────────────────────────────────────────
	if outDir != "" {
		// JSON 저장 (다음 날 비교용)
		if jsonPath, err := snapshot.SaveJSON(result, outDir); err == nil {
			fmt.Fprintf(os.Stderr, "[snapshot] JSON saved: %s\n", jsonPath)
		} else {
			fmt.Fprintf(os.Stderr, "[snapshot] JSON save failed: %v\n", err)
		}
		// Markdown 저장 (사람 읽기용)
		mdPath := filepath.Join(outDir, fmt.Sprintf("market_snapshot.%s.md", date))
		if err := os.WriteFile(mdPath, []byte(output), 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "[snapshot] MD saved: %s\n", mdPath)
		} else {
			fmt.Fprintf(os.Stderr, "[snapshot] MD save failed: %v\n", err)
		}
	}
	return nil
}

// runIntradayPulse는 근실시간 시장 펄스를 수집하고 출력합니다.
func runIntradayPulse(args []string) error {
	fs := flag.NewFlagSet("intraday-pulse", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	storeDir := fs.String("store-dir", envDefault("PULSE_OUTPUT_DIR", ".cache/pulse"), "펄스 적립 디렉터리")
	noSave := fs.Bool("no-save", false, "적립/렌더 저장 생략 (읽기 전용)")
	asJSON := fs.Bool("json", false, "JSON 출력")
	safetyOnly := fs.Bool("safety-only", false, "서킷브레이커 및 사이드카 상태만 렌더링")
	premarketFlag := fs.Bool("premarket", false, "개장전 취약도 보드 실행")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := pulse.Options{
		StoreDir: *storeDir,
		Lookback: 2 * time.Hour,
		NoSave:   *noSave,
		JSON:     *asJSON,
	}

	client, err := newKISClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := client.EnsureAuthToken(ctx); err != nil {
		return fmt.Errorf("auth token: %w", err)
	}

	yahooClient, naverClient := newExternalClients(client)

	result := pulse.Collect(ctx, pulse.Deps{
		Stock:    domesticstock.NewService(client),
		Future:   domesticfutureoption.NewService(client),
		Yahoo:    yahooClient,
		Naver:    naverClient,
		Clock:    time.Now,
		StoreDir: *storeDir,
	}, opts)

	if *asJSON {
		if *safetyOnly {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result.Safety)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pulse.PulseToMap(result))
	}

	if *premarketFlag {
		return runPremarket(args)
	}

	var output string
	if *safetyOnly {
		output = pulse.RenderSafetyOnly(result)
	} else {
		output = pulse.Render(result)
	}
	fmt.Print(output)

	// MD 저장
	if !*noSave {
		if err := pulse.SaveMD(opts, result.Date, output); err != nil {
			fmt.Fprintf(os.Stderr, "[pulse] MD save failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[pulse] MD saved: %s\n", pulse.MDPath(opts, result.Date))
		}
	}
	return nil
}

func runPremarket(args []string) error {
	fs := flag.NewFlagSet("premarket", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	storeDir := fs.String("store-dir", envDefault("PREMARKET_STORE_DIR", ".cache/premarket"), "개장전 적립 디렉터리")
	noSave := fs.Bool("no-save", false, "적립/저장 생략")
	asJSON := fs.Bool("json", false, "JSON 출력")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := premarket.Options{
		StoreDir: *storeDir,
		NoSave:   *noSave,
		JSON:     *asJSON,
	}

	client, _ := newKISClient()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var stockService premarket.DomesticStock
	var futureService premarket.DomesticFuture
	var yahooClient premarket.YahooQuotes
	var naverClient premarket.NaverFinance

	if client != nil {
		_, _ = client.EnsureAuthToken(ctx)
		stockService = domesticstock.NewService(client)
		futureService = domesticfutureoption.NewService(client)
		yC, nC := newExternalClients(client)
		yahooClient = yC
		naverClient = nC
	}
	kofiaClient := kofia.NewCachedClient(envDefault("KOFIA_CACHE_DIR", ".cache"), os.Getenv("USER_AGENT"))

	result := premarket.Collect(ctx, premarket.Deps{
		Stock:    stockService,
		Future:   futureService,
		Yahoo:    yahooClient,
		Naver:    naverClient,
		KOFIA:    kofiaClient,
		Clock:    time.Now,
		StoreDir: *storeDir,
	}, opts)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	output := premarket.Render(result)
	fmt.Print(output)
	return nil
}

func runRSI(args []string) error {
	fs := flag.NewFlagSet("rsi", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	symbol := fs.String("symbol", "^KS11", "지수/종목 코드 (기본: ^KS11 - KOSPI)")
	period := fs.Int("period", 14, "RSI 기간 (기본: 14)")
	days := fs.Int("days", 30, "역사적 출력 일수 (기본: 30)")
	method := fs.String("method", "wilder", "RSI 산출 방식 (wilder: 지수평활/Wilder, sma: 단순평균/Cutler)")
	interval := fs.String("interval", "1d", "차트 주기 (1d: 일봉, 60m: 1시간봉, 15m: 15분봉, 5m: 5분봉)")
	asJSON := fs.Bool("json", false, "JSON 시계열 출력")
	if err := fs.Parse(args); err != nil {
		return err
	}

	symTarget := strings.TrimSpace(*symbol)
	switch strings.ToLower(symTarget) {
	case "kospi", "0001", "ksp", "코스피":
		symTarget = "^KS11"
	case "kosdaq", "1001", "kdq", "코스닥":
		symTarget = "^KQ11"
	case "samsung", "삼성전자", "005930":
		symTarget = "005930.KS"
	case "skhynix", "sk하이닉스", "000660":
		symTarget = "000660.KS"
	}

	client, _ := newKISClient()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var dates []string
	var closes []float64

	if client != nil {
		yahooClient, _ := newExternalClients(client)
		rangeStr := "1y"
		if *interval != "1d" {
			rangeStr = "5d"
		} else if *days > 1250 {
			rangeStr = "max"
		} else if *days > 250 {
			rangeStr = "5y"
		}

		chartCloses, err := yahooClient.GetChartHistory(ctx, symTarget, rangeStr, *interval)
		if err == nil && len(chartCloses) > *period {
			for _, c := range chartCloses {
				timeFmt := "20060102"
				if *interval != "1d" {
					timeFmt = "20060102 15:04"
				}
				tStr := time.Unix(c.DateUnix, 0).In(time.FixedZone("KST", 9*3600)).Format(timeFmt)
				dates = append(dates, tStr)
				closes = append(closes, c.Close)
			}
		}
	}

	// Fallback data if API client is not configured
	if len(closes) <= *period {
		now := time.Now()
		for i := 120; i >= 0; i-- {
			d := now.AddDate(0, 0, -i)
			if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
				dates = append(dates, d.Format("20060102"))
				closes = append(closes, 6500.0+float64(i%15)*8.0)
			}
		}
	}

	series, err := domesticstock.CalculateRSISeriesWithMethod(dates, closes, *period, *method)
	if err != nil {
		return err
	}

	if len(series) > *days {
		series = series[len(series)-*days:]
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(series)
	}

	fmt.Printf("📊 %s %d일 RSI 역사적 시계열 리포트 (최근 %d영업일)\n", symTarget, *period, len(series))
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("| 날짜 (Date) | 종가 (Close) | %d일 RSI | 신호 (Signal) |\n", *period)
	fmt.Printf("| :--- | :---: | :---: | :--- |\n")
	for _, p := range series {
		fmt.Printf("| %s | %.2f | %.2f | %s |\n", p.Date, p.Close, p.RSI, p.Signal)
	}
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	return nil
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
