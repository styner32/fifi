package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/parse"
)

// CreditBalanceReport는 종목별 신용잔고 출력 구조체입니다.
type CreditBalanceReport struct {
	Symbol      string             `json:"symbol"`
	Name        string             `json:"name"`
	GeneratedAt time.Time          `json:"generated_at"`
	Price       PriceInfo          `json:"price"`
	Summary     CreditSummary      `json:"summary"`
	Daily       []CreditBalanceDay `json:"daily"`
}

type PriceInfo struct {
	CurrentPrice  int64   `json:"current_price"`
	PreviousClose int64   `json:"previous_close"`
	ChangePercent float64 `json:"change_percent"`
	Volume        int64   `json:"volume"`
}

type CreditSummary struct {
	BalanceQty    int64   `json:"balance_qty"`
	BalanceAmtEok float64 `json:"balance_amt_eok"`
	CreditRatePct float64 `json:"credit_rate_pct"`
	Trend5D       string  `json:"trend_5d"`
}

type CreditBalanceDay struct {
	Date       string `json:"date"`
	NewQty     int64  `json:"new_qty"`
	RepayQty   int64  `json:"repay_qty"`
	BalanceQty int64  `json:"balance_qty"`
	BalanceAmt int64  `json:"balance_amt"`
	NetNew     int64  `json:"net_new"`
	CreditRate string `json:"credit_rate"`
}

func runCreditBalance(args []string) error {
	for _, arg := range args {
		if arg == "--market-leverage" || arg == "--ratio" || arg == "--leverage" {
			return runMarketLeverageRatio(args)
		}
	}

	symbols, err := parseCreditArgs(args)
	if err != nil {
		return err
	}
	if len(symbols) == 0 {
		return runMarketLeverageRatio(args)
	}

	client, err := newKISClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := client.EnsureAuthToken(ctx); err != nil {
		return fmt.Errorf("auth token: %w", err)
	}

	svc := domesticstock.NewService(client)
	outDir := envDefault("CREDIT_BALANCE_OUTPUT_DIR", ".cache/credit_balance")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	date := kst.Now().Format("20060102")
	var reports []CreditBalanceReport

	for i, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "[credit-balance] [%d/%d] %s: fetching...\n", i+1, len(symbols), symbol)

		report, err := buildCreditReport(ctx, svc, symbol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[credit-balance] %s: ERROR: %v\n", symbol, err)
			continue
		}
		reports = append(reports, *report)
		fmt.Fprintf(os.Stderr, "[credit-balance] %s (%s): 잔고 %s주, %.1f억, 잔고율 %.2f%%\n",
			symbol, report.Name,
			formatInt(report.Summary.BalanceQty),
			report.Summary.BalanceAmtEok,
			report.Summary.CreditRatePct,
		)

		time.Sleep(300 * time.Millisecond)
	}

	if len(reports) > 0 {
		path := fmt.Sprintf("%s/credit_balance.%s.json", outDir, date)
		data, _ := json.MarshalIndent(reports, "", "  ")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "[credit-balance] save error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[credit-balance] saved: %s\n", path)
		}
	}

	printCreditTable(reports)
	return nil
}

func buildCreditReport(ctx context.Context, svc *domesticstock.Service, symbol string) (*CreditBalanceReport, error) {
	var name string
	var currentPrice, prevClose, volume int64
	var changePct float64

	priceResp, priceErr := svc.InquirePrice(ctx, symbol)
	if priceErr == nil {
		priceOutput := firstOutputMap(priceResp, "output")
		name = strVal(priceOutput, "hts_kor_isnm")
		currentPrice = intVal(priceOutput, "stck_prpr")
		prevClose = intVal(priceOutput, "stck_sdpr")
		volume = intVal(priceOutput, "acml_vol")
		if prevClose > 0 {
			changePct = float64(currentPrice-prevClose) / float64(prevClose) * 100
		}
	}

	time.Sleep(300 * time.Millisecond)

	creditResp, err := svc.InquireDailyCreditBalance(ctx, symbol, "")
	if err != nil {
		return nil, fmt.Errorf("credit: %w", err)
	}

	rows := outputArray(creditResp, "output")
	if len(rows) == 0 {
		rows = outputArray(creditResp, "output1")
	}
	if len(rows) == 0 {
		rows = outputArray(creditResp, "output2")
	}

	var daily []CreditBalanceDay
	for _, row := range rows {
		d := CreditBalanceDay{
			Date:       strVal(row, "deal_date"),
			NewQty:     intVal(row, "whol_loan_new_stcn"),
			RepayQty:   intVal(row, "whol_loan_rdmp_stcn"),
			BalanceQty: intVal(row, "whol_loan_rmnd_stcn"),
			BalanceAmt: intVal(row, "whol_loan_rmnd_amt"),
			CreditRate: strVal(row, "whol_loan_rmnd_rate"),
		}
		d.NetNew = d.NewQty - d.RepayQty
		if d.Date != "" {
			daily = append(daily, d)
		}
	}

	var summary CreditSummary
	if len(daily) > 0 {
		latest := daily[0]
		summary.BalanceQty = latest.BalanceQty
		summary.BalanceAmtEok = float64(latest.BalanceAmt) / 1e8
		summary.BalanceAmtEok = math.Round(summary.BalanceAmtEok*10) / 10
		if rate, err := strconv.ParseFloat(latest.CreditRate, 64); err == nil {
			summary.CreditRatePct = rate
		}
		if len(daily) >= 2 {
			oldest := daily[len(daily)-1]
			if len(daily) > 5 {
				oldest = daily[4]
			}
			diff := latest.BalanceQty - oldest.BalanceQty
			switch {
			case diff > 0:
				summary.Trend5D = "증가"
			case diff < 0:
				summary.Trend5D = "감소"
			default:
				summary.Trend5D = "보합"
			}
		}
	}

	return &CreditBalanceReport{
		Symbol:      symbol,
		Name:        name,
		GeneratedAt: time.Now(),
		Price: PriceInfo{
			CurrentPrice:  currentPrice,
			PreviousClose: prevClose,
			ChangePercent: math.Round(changePct*100) / 100,
			Volume:        volume,
		},
		Summary: summary,
		Daily:   daily,
	}, nil
}

func printCreditTable(reports []CreditBalanceReport) {
	if len(reports) == 0 {
		fmt.Println("\n신용잔고 조회 결과가 없습니다.")
		return
	}
	fmt.Println()
	fmt.Println("# 종목별 신용잔고 요약")
	fmt.Println()
	fmt.Println("| 종목 | 현재가 | 등락률 | 신용잔고(주) | 잔고금액(억) | 잔고율(%) | 5일추세 |")
	fmt.Println("|---|---|---|---|---|---|---|")
	for _, r := range reports {
		fmt.Printf("| %s %s | %s | %+.2f%% | %s | %.1f | %.2f%% | %s |\n",
			r.Symbol, r.Name,
			formatInt(r.Price.CurrentPrice),
			r.Price.ChangePercent,
			formatInt(r.Summary.BalanceQty),
			r.Summary.BalanceAmtEok,
			r.Summary.CreditRatePct,
			r.Summary.Trend5D,
		)
	}
	fmt.Println()
}

func parseCreditArgs(args []string) ([]string, error) {
	for i, arg := range args {
		if arg == "--watchlist" && i+1 < len(args) {
			return parseWatchlistFile(args[i+1])
		}
	}
	var symbols []string
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg != "" && !strings.HasPrefix(arg, "--") {
			symbols = append(symbols, arg)
		}
	}
	return symbols, nil
}

func parseWatchlistFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("watchlist: %w", err)
	}
	defer f.Close()

	var symbols []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			symbols = append(symbols, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("watchlist scan: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[credit-balance] watchlist: %d symbols from %s\n", len(symbols), path)
	return symbols, nil
}

func firstOutputMap(resp *auth.RESTResponse, key string) map[string]any {
	return resp.FirstRow(key)
}

func outputArray(resp *auth.RESTResponse, key string) []map[string]any {
	return resp.Rows(key)
}

func strVal(m map[string]any, key string) string {
	if m == nil || m[key] == nil {
		return ""
	}
	return strings.TrimSpace(parse.String(m[key]))
}

func intVal(m map[string]any, key string) int64 {
	v, ok := parse.Num(m, key)
	if !ok {
		return 0
	}
	return int64(v)
}

func formatInt(v int64) string {
	s := strconv.FormatInt(v, 10)
	if v < 0 {
		s = s[1:]
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	out := string(result)
	if v < 0 {
		out = "-" + out
	}
	return out
}
