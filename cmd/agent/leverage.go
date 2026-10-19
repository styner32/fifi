package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/kofia"
	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/parse"
)

// Amounts are nullable: an unavailable observation must never appear as a zero.
type MarketLeveragePoint struct {
	Date                    string   `json:"date"`
	KOSPIPrice              float64  `json:"kospi_price"`
	KOSPIChangePct          *float64 `json:"kospi_change_pct"`
	CreditBalanceTrillion   *float64 `json:"credit_balance_trillion"`
	CreditChangePct         *float64 `json:"credit_change_pct"`
	CreditSrc               string   `json:"credit_src"`
	CustomerDepositTrillion *float64 `json:"customer_deposit_trillion"`
	DepositChangePct        *float64 `json:"deposit_change_pct"`
	DepositSrc              string   `json:"deposit_src"`
	CreditToDepositPct      *float64 `json:"credit_to_deposit_pct"`
	CreditToDepositChgP     *float64 `json:"credit_to_deposit_chg_p"`
	Status                  string   `json:"status"`
	MissingReason           string   `json:"missing_reason,omitempty"`
	Diagnosis               string   `json:"diagnosis"`
}

type marketFundsSource interface {
	MarketFunds(context.Context, string) (*auth.RESTResponse, error)
}

type marketFundsRange interface {
	GetMarketFunds(context.Context, string, string) ([]kofia.MarketFundsRow, error)
}

func runMarketLeverageRatio(args []string) error {
	var cleanArgs []string
	for _, arg := range args {
		if arg != "--ratio" && arg != "--market-leverage" && arg != "--leverage" {
			cleanArgs = append(cleanArgs, arg)
		}
	}
	fs := flag.NewFlagSet("market-leverage", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	days := fs.Int("days", 30, "출력할 관측 일수")
	asJSON := fs.Bool("json", false, "JSON 시계열 출력 (결측은 null)")
	if err := fs.Parse(cleanArgs); err != nil {
		return err
	}
	if *days <= 0 || *days > 250 {
		return fmt.Errorf("--days must be between 1 and 250")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var stock marketFundsSource
	if client, err := newKISClient(); err == nil {
		if _, err = client.EnsureAuthToken(ctx); err == nil {
			stock = domesticstock.NewService(client)
		} else {
			fmt.Fprintln(os.Stderr, "[leverage] KIS authentication unavailable; credit observations will be null")
		}
	}
	history := yahoo.NewClient(&http.Client{Timeout: 15 * time.Second}, yahoo.Config{UserAgent: os.Getenv("USER_AGENT")})
	points, warnings, err := collectMarketLeverage(ctx, history, stock, kofia.NewClient(os.Getenv("USER_AGENT")), *days, kst.Now())
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "[leverage] %s\n", warning)
	}
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(points)
	}
	fmt.Println("# 신용융자 잔고 / 투자자예탁금 — 동일 기준일 관측값")
	fmt.Println("\n| 기준일 | KOSPI | 신용잔고(조원) | 예탁금(조원) | 비율(%) | 상태 |")
	fmt.Println("|---|---:|---:|---:|---:|---|")
	for _, point := range points {
		fmt.Printf("| %s | %.2f | %s | %s | %s | %s |\n", point.Date, point.KOSPIPrice,
			leverageNumber(point.CreditBalanceTrillion), leverageNumber(point.CustomerDepositTrillion), leverageNumber(point.CreditToDepositPct), point.Status)
	}
	fmt.Println("\n출처별 기준일이 일치할 때만 비율을 계산합니다. 위험·투매·자금유출 모델은 NOT_EVALUATED입니다.")
	return nil
}

// One KIS history query and one KOFIA range query avoid per-day request storms.
func collectMarketLeverage(ctx context.Context, history priceHistory, stock marketFundsSource, funds marketFundsRange, days int, now time.Time) ([]MarketLeveragePoint, []string, error) {
	if days <= 0 || days > 250 {
		return nil, nil, fmt.Errorf("days must be between 1 and 250")
	}
	prices, err := history.GetChartHistory(ctx, "^KS11", "1y", "1d")
	if err != nil {
		return nil, nil, fmt.Errorf("leverage UNAVAILABLE: Yahoo history: %w", err)
	}
	prices, err = validatedPrices(prices, now)
	if err != nil || len(prices) == 0 {
		return nil, nil, fmt.Errorf("leverage UNAVAILABLE: valid price history required: %v", err)
	}
	// Retain one earlier bar for changes, but do not fabricate missing trading days.
	if len(prices) > days+1 {
		prices = prices[len(prices)-days-1:]
	}
	start := time.Unix(prices[0].DateUnix, 0).In(kst.Location).Format("20060102")
	end := now.In(kst.Location).Format("20060102")
	warnings := []string{}
	kisRows := map[string]map[string]any{}
	conflicts := map[string]bool{}
	if stock != nil {
		resp, fetchErr := stock.MarketFunds(ctx, end)
		if fetchErr != nil || resp == nil || !resp.IsOK() {
			warnings = append(warnings, "KIS market funds unavailable")
		} else {
			for _, row := range resp.Rows("output") {
				date := parse.String(row["bsop_date"])
				if _, dateErr := time.Parse("20060102", date); dateErr != nil || date > end {
					continue
				}
				if old, exists := kisRows[date]; exists {
					// Equal raw numeric observations are harmless duplicate rows.
					for _, key := range []string{"crdt_loan_rmnd", "cust_dpmn_amt"} {
						if parse.String(old[key]) != parse.String(row[key]) {
							conflicts[date] = true
						}
					}
				}
				kisRows[date] = row
			}
		}
	} else {
		warnings = append(warnings, "KIS credentials unavailable; credit history is unavailable")
	}
	kofiaRows := map[string]kofia.MarketFundsRow{}
	kofiaConflicts := map[string]bool{}
	if funds != nil {
		rows, fetchErr := funds.GetMarketFunds(ctx, start, end)
		if fetchErr != nil {
			warnings = append(warnings, "KOFIA deposit history unavailable")
		} else {
			for _, row := range rows {
				if _, dateErr := time.Parse("20060102", row.Date); dateErr != nil || row.Date > end || row.AmountUnit != "KRW_MILLION" {
					continue
				}
				if old, exists := kofiaRows[row.Date]; exists && old.CustomerDepositMln != row.CustomerDepositMln {
					kofiaConflicts[row.Date] = true
				}
				kofiaRows[row.Date] = row
			}
		}
	}
	points := make([]MarketLeveragePoint, 0, len(prices))
	for _, price := range prices {
		date := time.Unix(price.DateUnix, 0).In(kst.Location).Format("20060102")
		p := MarketLeveragePoint{Date: date, KOSPIPrice: price.Close, CreditSrc: "UNAVAILABLE", DepositSrc: "UNAVAILABLE", Status: "UNAVAILABLE", Diagnosis: "NOT_EVALUATED"}
		if row := kisRows[date]; row != nil && !conflicts[date] {
			if value, ok := parse.Float(row["crdt_loan_rmnd"]); ok {
				p.CreditBalanceTrillion = nonnegativeAmount(value, 10000) // KIS: 억원 -> 조원
				if p.CreditBalanceTrillion != nil {
					p.CreditSrc = "KIS_MKTFUNDS_KRW_100M"
				}
			}
			if value, ok := parse.Float(row["cust_dpmn_amt"]); ok {
				p.CustomerDepositTrillion = nonnegativeAmount(value, 10000)
				if p.CustomerDepositTrillion != nil {
					p.DepositSrc = "KIS_MKTFUNDS_KRW_100M"
				}
			}
		}
		if row, ok := kofiaRows[date]; ok && p.CustomerDepositTrillion == nil && !kofiaConflicts[date] {
			p.CustomerDepositTrillion = nonnegativeAmount(row.CustomerDepositMln, 1000000) // million KRW -> trillion KRW
			if p.CustomerDepositTrillion != nil {
				p.DepositSrc = "KOFIA_KRW_MILLION"
			}
		}
		if p.CreditBalanceTrillion != nil && p.CustomerDepositTrillion != nil && *p.CustomerDepositTrillion > 0 {
			p.CreditToDepositPct = leveragePtr(*p.CreditBalanceTrillion / *p.CustomerDepositTrillion * 100)
			p.Status = "LAGGED_CONTEXT_ONLY"
		} else {
			p.MissingReason = "same-date credit and positive deposit observations required; publication delay is unconfirmed"
		}
		if len(points) > 0 {
			previous := points[len(points)-1]
			p.KOSPIChangePct = changePercent(&p.KOSPIPrice, &previous.KOSPIPrice)
			p.CreditChangePct = changePercent(p.CreditBalanceTrillion, previous.CreditBalanceTrillion)
			p.DepositChangePct = changePercent(p.CustomerDepositTrillion, previous.CustomerDepositTrillion)
			if p.CreditToDepositPct != nil && previous.CreditToDepositPct != nil {
				p.CreditToDepositChgP = leveragePtr(*p.CreditToDepositPct - *previous.CreditToDepositPct)
			}
		}
		points = append(points, p)
	}
	if len(points) > days {
		points = points[len(points)-days:]
	}
	return points, warnings, ctx.Err()
}

func nonnegativeAmount(value, divisor float64) *float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return leveragePtr(value / divisor)
}

func leveragePtr(value float64) *float64 { return &value }

func changePercent(value, previous *float64) *float64 {
	if value == nil || previous == nil || *previous <= 0 {
		return nil
	}
	return leveragePtr((*value / *previous - 1) * 100)
}

func leverageNumber(value *float64) string {
	if value == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.2f", *value)
}
