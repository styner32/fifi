package companyanalysis

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/fifi/internal/marketpremium"
)

func (s *Service) fetchRiskFreeRate(ctx context.Context) (float64, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.FRED10YearURL, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", s.cfg.SECUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, false
	}

	reader := csv.NewReader(resp.Body)
	_, err = reader.Read()
	if err != nil {
		return 0, false
	}

	var last float64
	found := false
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			return 0, false
		}
		value := strings.TrimSpace(record[1])
		if value == "" || value == "." {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		last = parsed / 100
		found = true
	}
	return last, found
}

func (s *Service) resolveMarketPremium(ctx context.Context) (float64, string, bool) {
	value, err := marketpremium.Resolver{HTTPClient: s.httpClient}.Resolve(ctx)
	if err != nil || value == nil || value.Rate <= 0 {
		return 0, "", false
	}
	note := strings.TrimSpace(value.Note)
	if value.AsOf != "" {
		if note == "" {
			note = "as of " + value.AsOf
		} else {
			note = note + " | as of " + value.AsOf
		}
	}
	return value.Rate, note, true
}

func (s *Service) fetchStooqHistory(ctx context.Context, symbol string) ([]stooqRow, error) {
	url := strings.TrimRight(s.cfg.StooqBaseURL, "/") + "/?s=" + stooqSymbol(symbol) + "&i=" + stooqInterval
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", s.cfg.SECUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stooq status %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if len(headers) < 5 {
		return nil, fmt.Errorf("stooq CSV header is incomplete")
	}

	rows := make([]stooqRow, 0, 512)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 5 {
			continue
		}
		closeValue, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64)
		if err != nil || closeValue <= 0 {
			continue
		}
		rows = append(rows, stooqRow{
			Date:  strings.TrimSpace(record[0]),
			Close: closeValue,
		})
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("stooq returned too few rows for %s", symbol)
	}
	return rows, nil
}

func stooqSymbol(symbol string) string {
	symbol = strings.ToLower(strings.TrimSpace(symbol))
	if strings.Contains(symbol, ".") {
		return symbol
	}
	return symbol + ".us"
}

func buildQuoteFromHistory(symbol string, rows []stooqRow) (Quote, error) {
	if len(rows) < 2 {
		return Quote{}, fmt.Errorf("stooq history for %s is too short", symbol)
	}
	last := rows[len(rows)-1]
	prev := rows[len(rows)-2]
	change := last.Close - prev.Close
	changePercent := 0.0
	if prev.Close > 0 {
		changePercent = (change / prev.Close) * 100
	}
	return Quote{
		Symbol:        symbol,
		PriceDate:     last.Date,
		Price:         last.Close,
		PreviousClose: prev.Close,
		Change:        change,
		ChangePercent: changePercent,
		Currency:      "USD",
		Source:        "stooq daily close",
	}, nil
}

func deriveBetaFromHistories(stockRows []stooqRow, benchmarkRows []stooqRow, lookbackDays int) (float64, int, bool) {
	stockReturns := dailyReturnSeries(stockRows)
	benchmarkReturns := dailyReturnSeries(benchmarkRows)
	if len(stockReturns) == 0 || len(benchmarkReturns) == 0 {
		return 0, 0, false
	}

	dates := make([]string, 0, minInt(len(stockReturns), len(benchmarkReturns)))
	for date := range stockReturns {
		if _, ok := benchmarkReturns[date]; ok {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	if len(dates) > lookbackDays {
		dates = dates[len(dates)-lookbackDays:]
	}

	minObservations := minimumBetaObservations
	if lookbackDays > 0 && lookbackDays < minObservations {
		minObservations = lookbackDays
	}
	if minObservations < 30 {
		minObservations = 30
	}
	if len(dates) < minObservations {
		return 0, len(dates), false
	}

	stock := make([]float64, 0, len(dates))
	benchmark := make([]float64, 0, len(dates))
	for _, date := range dates {
		stock = append(stock, stockReturns[date])
		benchmark = append(benchmark, benchmarkReturns[date])
	}

	meanStock := averageFloatSlice(stock)
	meanBenchmark := averageFloatSlice(benchmark)
	var covariance float64
	var variance float64
	for i := range stock {
		stockDelta := stock[i] - meanStock
		benchmarkDelta := benchmark[i] - meanBenchmark
		covariance += stockDelta * benchmarkDelta
		variance += benchmarkDelta * benchmarkDelta
	}
	if variance == 0 {
		return 0, len(dates), false
	}
	beta := covariance / variance
	if math.IsNaN(beta) || math.IsInf(beta, 0) {
		return 0, len(dates), false
	}
	return beta, len(dates), true
}

func dailyReturnSeries(rows []stooqRow) map[string]float64 {
	if len(rows) < 2 {
		return nil
	}
	returns := make(map[string]float64, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1].Close
		if prev <= 0 {
			continue
		}
		returns[rows[i].Date] = (rows[i].Close / prev) - 1
	}
	return returns
}

func deriveCostOfDebt(latest AnnualRecord, financials []AnnualRecord) (float64, bool) {
	if latest.InterestExpense <= 0 || latest.TotalDebt <= 0 {
		return 0, false
	}
	avgDebt := latest.TotalDebt
	if len(financials) > 1 && financials[1].TotalDebt > 0 {
		avgDebt = (latest.TotalDebt + financials[1].TotalDebt) / 2
	}
	if avgDebt <= 0 {
		return 0, false
	}
	costOfDebt := latest.InterestExpense / avgDebt
	if costOfDebt <= 0 || costOfDebt > 0.30 {
		return 0, false
	}
	return costOfDebt, true
}
