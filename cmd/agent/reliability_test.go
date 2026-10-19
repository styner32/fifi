package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/external/kofia"
	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/kst"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type capturedHistory struct {
	rows []yahoo.DailyClose
	err  error
}

func (f capturedHistory) GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error) {
	return f.rows, f.err
}

type capturedMarketFunds struct{ rows []map[string]any }

func (f capturedMarketFunds) MarketFunds(context.Context, string) (*auth.RESTResponse, error) {
	rows := make([]any, len(f.rows))
	for i, row := range f.rows {
		rows[i] = row
	}
	return &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output": rows}}, nil
}

type capturedFundsRange struct{ rows []kofia.MarketFundsRow }

func (f capturedFundsRange) GetMarketFunds(context.Context, string, string) ([]kofia.MarketFundsRow, error) {
	return f.rows, nil
}

var _ = Describe("Evidence-bound technical reports", func() {
	var now time.Time
	BeforeEach(func() { now = time.Date(2026, 10, 18, 12, 0, 0, 0, kst.Location) })

	Describe("RSI observations", func() {
		Context("when an unsupported method is supplied", func() {
			It("rejects the method before requesting any prices", func() {
				points, err := fetchRSISeries(context.Background(), nil, "^KS11", "1d", 14, 30, "typo", now)
				Expect(err).To(MatchError(ContainSubstring("method must be")))
				Expect(points).To(BeNil())
			})
		})
		Context("when prices are unavailable", func() {
			DescribeTable("returns no manufactured series", func(history capturedHistory) {
				points, err := fetchRSISeries(context.Background(), history, "^KS11", "1d", 14, 30, "wilder", now)
				Expect(err).To(MatchError(ContainSubstring("UNAVAILABLE")))
				Expect(points).To(BeNil())
			},
				Entry("upstream failure", capturedHistory{err: errors.New("offline")}),
				Entry("insufficient observations", capturedHistory{rows: []yahoo.DailyClose{{DateUnix: now.Add(-time.Hour).Unix(), Close: 200}}}),
			)
		})
		Context("when actual prices are provided without KIS configuration", func() {
			It("uses only supplied observations and retains source dates", func() {
				rows := []yahoo.DailyClose{}
				for i := 20; i > 0; i-- {
					rows = append(rows, yahoo.DailyClose{DateUnix: now.AddDate(0, 0, -i).Unix(), Close: 200 + float64(20-i)})
				}
				points, err := fetchRSISeries(context.Background(), capturedHistory{rows: rows}, "^KS11", "1d", 14, 3, "wilder", now)
				Expect(err).NotTo(HaveOccurred())
				Expect(points).To(HaveLen(3))
				Expect(points[2].Close).To(Equal(219.0))
				Expect(points[2].Date).To(Equal("20261017"))
			})
		})
		DescribeTable("rejects invalid prices", func(row yahoo.DailyClose) {
			_, err := validatedPrices([]yahoo.DailyClose{row}, now)
			Expect(err).To(HaveOccurred())
		},
			Entry("future timestamp", yahoo.DailyClose{DateUnix: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), Close: 10}),
			Entry("missing timestamp", yahoo.DailyClose{Close: 10}),
			Entry("negative price", yahoo.DailyClose{DateUnix: 100, Close: -1}),
			Entry("non-finite price", yahoo.DailyClose{DateUnix: 100, Close: math.Inf(1)}),
		)
	})

	Describe("Credit to deposit observations", func() {
		var history capturedHistory
		BeforeEach(func() {
			history = capturedHistory{rows: []yahoo.DailyClose{
				{DateUnix: time.Date(2026, 10, 15, 15, 30, 0, 0, kst.Location).Unix(), Close: 200},
				{DateUnix: time.Date(2026, 10, 16, 15, 30, 0, 0, kst.Location).Unix(), Close: 202},
			}}
		})
		Context("with published dates after the former fixed cutoff", func() {
			It("uses dated KIS values with explicit hundred-million to trillion conversion", func() {
				stock := capturedMarketFunds{rows: []map[string]any{{"bsop_date": "20261016", "crdt_loan_rmnd": "400000", "cust_dpmn_amt": "2000000"}}}
				points, _, err := collectMarketLeverage(context.Background(), history, stock, nil, 2, now)
				Expect(err).NotTo(HaveOccurred())
				Expect(points[1].CreditBalanceTrillion).To(HaveValue(Equal(40.0)))
				Expect(points[1].CustomerDepositTrillion).To(HaveValue(Equal(200.0)))
				Expect(points[1].CreditToDepositPct).To(HaveValue(Equal(20.0)))
				Expect(points[1].Diagnosis).To(Equal("NOT_EVALUATED"))
			})
		})
		Context("when credit is missing", func() {
			It("preserves same-day KOFIA deposits, null credit and unavailable ratio", func() {
				funds := capturedFundsRange{rows: []kofia.MarketFundsRow{{Date: "20261016", AmountUnit: "KRW_MILLION", CustomerDepositMln: 300000000}}}
				points, _, err := collectMarketLeverage(context.Background(), history, nil, funds, 2, now)
				Expect(err).NotTo(HaveOccurred())
				Expect(points[1].CustomerDepositTrillion).To(HaveValue(Equal(300.0)))
				Expect(points[1].CreditBalanceTrillion).To(BeNil())
				Expect(points[1].CreditToDepositPct).To(BeNil())
				Expect(points[1].Status).To(Equal("UNAVAILABLE"))
				raw, marshalErr := json.Marshal(points[1])
				Expect(marshalErr).NotTo(HaveOccurred())
				Expect(string(raw)).To(ContainSubstring(`"credit_balance_trillion":null`))
			})
		})
		Context("with different reference dates or unknown units", func() {
			DescribeTable("does not relabel or guess amounts", func(row kofia.MarketFundsRow) {
				points, _, err := collectMarketLeverage(context.Background(), history, nil, capturedFundsRange{rows: []kofia.MarketFundsRow{row}}, 2, now)
				Expect(err).NotTo(HaveOccurred())
				Expect(points[1].CustomerDepositTrillion).To(BeNil())
			},
				Entry("older source date", kofia.MarketFundsRow{Date: "20261014", AmountUnit: "KRW_MILLION", CustomerDepositMln: 100000000}),
				Entry("unknown amount unit", kofia.MarketFundsRow{Date: "20261016", CustomerDepositMln: 100000000}),
			)
		})
		Context("when source explicitly reports zero credit", func() {
			It("keeps the zero observation and calculates a zero ratio", func() {
				stock := capturedMarketFunds{rows: []map[string]any{{"bsop_date": "20261016", "crdt_loan_rmnd": "0", "cust_dpmn_amt": "1000000"}}}
				points, _, err := collectMarketLeverage(context.Background(), history, stock, nil, 2, now)
				Expect(err).NotTo(HaveOccurred())
				Expect(points[1].CreditBalanceTrillion).To(HaveValue(Equal(0.0)))
				Expect(points[1].CreditToDepositPct).To(HaveValue(Equal(0.0)))
			})
		})
		Context("when a date has conflicting source values", func() {
			It("withholds the conflicting observation", func() {
				stock := capturedMarketFunds{rows: []map[string]any{
					{"bsop_date": "20261016", "crdt_loan_rmnd": "100000", "cust_dpmn_amt": "1000000"},
					{"bsop_date": "20261016", "crdt_loan_rmnd": "200000", "cust_dpmn_amt": "1000000"},
				}}
				points, _, err := collectMarketLeverage(context.Background(), history, stock, nil, 2, now)
				Expect(err).NotTo(HaveOccurred())
				Expect(points[1].CreditBalanceTrillion).To(BeNil())
			})
		})
	})
})
