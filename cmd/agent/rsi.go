package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/kst"
)

type priceHistory interface {
	GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error)
}

func runRSI(args []string) error {
	fs := flag.NewFlagSet("rsi", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	symbol := fs.String("symbol", "^KS11", "지수/종목 코드")
	period := fs.Int("period", 14, "RSI 기간")
	days := fs.Int("days", 30, "출력할 관측 일수")
	method := fs.String("method", "wilder", "wilder 또는 sma")
	interval := fs.String("interval", "1d", "차트 주기")
	asJSON := fs.Bool("json", false, "JSON 시계열 출력")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *days <= 0 || *period <= 0 {
		return fmt.Errorf("--days and --period must be positive")
	}
	sym := strings.TrimSpace(*symbol)
	switch strings.ToLower(sym) {
	case "kospi", "0001", "ksp", "코스피":
		sym = "^KS11"
	case "kosdaq", "1001", "kdq", "코스닥":
		sym = "^KQ11"
	case "samsung", "삼성전자", "005930":
		sym = "005930.KS"
	case "skhynix", "sk하이닉스", "000660":
		sym = "000660.KS"
	}
	if sym == "" {
		return fmt.Errorf("--symbol is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := yahoo.NewClient(&http.Client{Timeout: 15 * time.Second}, yahoo.Config{UserAgent: os.Getenv("USER_AGENT")})
	series, err := fetchRSISeries(ctx, client, sym, *interval, *period, *days, *method, kst.Now())
	if err != nil {
		return err
	}
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(series)
	}
	fmt.Printf("# %s %d기간 RSI — Yahoo 관측값 (%s, %s)\n\n", sym, *period, *interval, *method)
	fmt.Println("| 날짜 | 가격 | RSI | 신호 |")
	fmt.Println("|---|---:|---:|---|")
	for _, p := range series {
		fmt.Printf("| %s | %.2f | %.2f | %s |\n", p.Date, p.Close, p.RSI, p.Signal)
	}
	return nil
}

func fetchRSISeries(ctx context.Context, client priceHistory, symbol, interval string, period, days int, method string, now time.Time) ([]domesticstock.RSIPoint, error) {
	if period <= 0 || days <= 0 {
		return nil, fmt.Errorf("RSI period and days must be positive")
	}
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case "wilder", "sma", "cutler":
	default:
		return nil, fmt.Errorf("RSI method must be wilder, sma, or cutler")
	}
	rangeStr := "1y"
	if interval != "1d" {
		rangeStr = "5d"
	} else if days > 1250 {
		rangeStr = "max"
	} else if days > 250 {
		rangeStr = "5y"
	}
	rows, err := client.GetChartHistory(ctx, symbol, rangeStr, interval)
	if err != nil {
		return nil, fmt.Errorf("RSI UNAVAILABLE: Yahoo history: %w", err)
	}
	rows, err = validatedPrices(rows, now)
	if err != nil {
		return nil, fmt.Errorf("RSI UNAVAILABLE: %w", err)
	}
	if len(rows) <= period {
		return nil, fmt.Errorf("RSI UNAVAILABLE: need more than %d valid observations, received %d", period, len(rows))
	}
	format := "20060102"
	if interval != "1d" {
		format = "20060102 15:04"
	}
	dates, closes := make([]string, 0, len(rows)), make([]float64, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, time.Unix(row.DateUnix, 0).In(kst.Location).Format(format))
		closes = append(closes, row.Close)
	}
	series, err := domesticstock.CalculateRSISeriesWithMethod(dates, closes, period, method)
	if err != nil {
		return nil, err
	}
	if len(series) > days {
		series = series[len(series)-days:]
	}
	return series, nil
}

// Reject conflicting or invalid observations instead of constructing substitute prices.
func validatedPrices(rows []yahoo.DailyClose, now time.Time) ([]yahoo.DailyClose, error) {
	byTime := make(map[int64]float64)
	for _, row := range rows {
		if row.DateUnix <= 0 || row.DateUnix > now.Unix() || row.Close <= 0 || math.IsNaN(row.Close) || math.IsInf(row.Close, 0) {
			return nil, fmt.Errorf("invalid or future price observation")
		}
		if old, exists := byTime[row.DateUnix]; exists && old != row.Close {
			return nil, fmt.Errorf("conflicting prices at the same source timestamp")
		}
		byTime[row.DateUnix] = row.Close
	}
	out := make([]yahoo.DailyClose, 0, len(byTime))
	for ts, close := range byTime {
		out = append(out, yahoo.DailyClose{DateUnix: ts, Close: close})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DateUnix < out[j].DateUnix })
	return out, nil
}
