package snapshot

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/kofia"
	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/kst"
)

func captured(key string) *auth.RESTResponse {
	GinkgoHelper()
	raw, err := os.ReadFile("testdata/kis_20260915.json")
	Expect(err).NotTo(HaveOccurred())
	var data map[string]map[string]any
	Expect(json.Unmarshal(raw, &data)).To(Succeed())
	Expect(data).To(HaveKey(key))
	return &auth.RESTResponse{Body: data[key]}
}

func obj(value any) map[string]any {
	GinkgoHelper()
	raw, err := json.Marshal(value)
	Expect(err).NotTo(HaveOccurred())
	var result map[string]any
	Expect(json.Unmarshal(raw, &result)).To(Succeed())
	return result
}

func response(key string, rows ...map[string]any) *auth.RESTResponse {
	values := make([]any, len(rows))
	for i, row := range rows {
		values[i] = row
	}
	return &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", key: values}}
}

type indexStock struct{ fakeStock }

func (s indexStock) KOSPIIndexMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error) {
	return s.cap, nil
}

func testCap() *domesticstock.KOSPIMarketCapSummary {
	result := &domesticstock.KOSPIMarketCapSummary{BusinessDate: "20260915", Universe: domesticstock.KOSPIIndexMasterUniverse, WeightStatus: domesticstock.KOSPIIndexWeightStatus, TotalMarketCap: 66}
	for i := 1; i <= 11; i++ {
		code := string(rune('A' + i))
		if i == 11 {
			code = "005930"
		}
		if i == 10 {
			code = "000660"
		}
		result.Constituents = append(result.Constituents, domesticstock.KOSPIMarketCapConstituent{Code: code, MarketCap: float64(i), KOSPIIndexMember: true, PreferredClass: "0"})
	}
	return result
}

type historyStock struct {
	fakeStock
	queryDate *string
}

func (s historyStock) InquireVKOSPIDailyPrice(_ context.Context, _, date string) ([]map[string]any, error) {
	*s.queryDate = date
	return s.vkospiDaily, nil
}

type fundsStock struct {
	fakeStock
	funds *auth.RESTResponse
}

func (s fundsStock) MarketFunds(context.Context, string) (*auth.RESTResponse, error) {
	return s.funds, nil
}

type fixedKofia struct{ row *kofia.MarketFundsRow }

func (s fixedKofia) GetMarketFundsForDate(context.Context, string) (*kofia.MarketFundsRow, error) {
	return s.row, nil
}

var _ = Describe("Snapshot data integrity", func() {
	var ctx context.Context
	BeforeEach(func() { ctx = context.Background() })

	Context("when collecting dated investor flows", func() {
		var stock fakeStock
		BeforeEach(func() { stock = fakeStock{investor: captured("inquire-investor-daily-by-market")} })

		It("converts all participant amounts to eok and retains the rounding residual", func() {
			flow, err := collectFlow(ctx, stock, "20260915")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow.ForeignEok).To(BeNumerically("~", -15710.70, 1e-7))
			Expect(flow.InstitutionEok).To(BeNumerically("~", -9038.97, 1e-7))
			Expect(flow.OtherCorporateEok).To(HaveValue(BeNumerically("~", 16430.47, 1e-7)))
			Expect(flow.AllParticipantResidualEok).To(HaveValue(BeNumerically("~", -0.01, 1e-7)))
		})

		It("uses the previous session from the same response instead of an older saved observation", func() {
			flow, err := collectFlow(ctx, stock, "20260915")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow.PreviousDate).To(Equal("20260914"))
			Expect(flow.ForeignPrevEok).To(HaveValue(BeNumerically("~", -33375.04, 1e-7)))
			output := Render(&Snapshot{Flow: flow}, &SnapshotJSON{Flow: &FlowSection{ForeignEok: -1, ForeignPrevEok: ptr(-2)}})
			Expect(output).To(ContainSubstring("-33,375"))
			Expect(output).NotTo(ContainSubstring("잔차: -0억원"))
		})

		DescribeTable("rejects a requested date that is absent from the response", func(date string) {
			flow, err := collectFlow(ctx, stock, date)
			Expect(err).To(HaveOccurred())
			Expect(flow).To(BeNil())
		}, Entry("a non-trading date", "20260912"), Entry("a later date", "20260916"))

		It("rejects a business error even when numeric rows are present", func() {
			stock.investor.Body["rt_cd"] = "1"
			flow, err := collectFlow(ctx, stock, "20260915")
			Expect(err).To(HaveOccurred())
			Expect(flow).To(BeNil())
		})
	})

	Context("when collecting program trades", func() {
		var section *LateSessionSection
		var stock fakeStock
		BeforeEach(func() {
			section = &LateSessionSection{BusinessDate: "20260915"}
			stock = fakeStock{program: captured("investor-program-trade-today"), compProg: captured("comp-program-trade-today")}
		})

		It("recognizes the spaced institution aggregate without promoting investor subtotals to market totals", func() {
			Expect(fillProgramTradeToday(ctx, Deps{DomesticStock: stock}, section)).To(Succeed())
			Expect(section.InstitutionOK).To(BeTrue())
			Expect(section.ForeignOK).To(BeTrue())
			Expect(section.KOSPINetNonArbitrageOrgan).To(BeNumerically("~", -714.10, 1e-7))
			Expect(section.KOSPINetNonArbitrageForeign).To(BeNumerically("~", -7015.03, 1e-7))
			Expect(section.ProgramOK).To(BeFalse())
		})

		It("reads non-overlapping market totals without claiming dated interval reconciliation", func() {
			Expect(fillProgramTradeToday(ctx, Deps{DomesticStock: stock}, section)).To(Succeed())
			Expect(fillLateProgramFlow(ctx, Deps{DomesticStock: stock}, section, Options{})).To(Succeed())
			Expect(section.KOSPINetArbitrageTotal).To(BeNumerically("~", -870.13, 1e-7))
			Expect(section.KOSPINetNonArbitrageTotal).To(BeNumerically("~", -8175.39, 1e-7))
			Expect(section.KOSPIProgramTotalNet).To(BeNumerically("~", -9045.51, 1e-7))
			Expect(section.ProgramOK).To(BeTrue())
			Expect(section.ProgramSourceTime).To(Equal("154200"))
			Expect(section.CanonicalTotalEok).To(BeNil())
			Expect(section.LateProgramNetEok).To(BeNil())
			Expect(section.CloseSessionProgramNetEok).To(BeNil())
		})

		DescribeTable("preserves missing institution amounts as null", func(field string) {
			row := map[string]any{"invr_cls_code": "8888", "invr_cls_name": "기 관", "arbt_ntby_amt": "-10", "nabt_ntby_amt": "-20", "all_ntby_amt": "-30"}
			delete(row, field)
			stock.program = response("output1", row)
			Expect(fillProgramTradeToday(ctx, Deps{DomesticStock: stock}, section)).To(Succeed())
			Expect(section.InstitutionOK).To(BeFalse())
			Expect(obj(section)).To(HaveKeyWithValue("kospi_net_non_arbitrage_organ", BeNil()))
		}, Entry("missing arbitrage", "arbt_ntby_amt"), Entry("missing non-arbitrage", "nabt_ntby_amt"), Entry("missing total", "all_ntby_amt"))

		It("retains an explicitly reported zero", func() {
			stock.program = response("output1", map[string]any{"invr_cls_code": "8888", "invr_cls_name": "기 관", "arbt_ntby_amt": "0", "nabt_ntby_amt": "0", "all_ntby_amt": "0"})
			Expect(fillProgramTradeToday(ctx, Deps{DomesticStock: stock}, section)).To(Succeed())
			Expect(section.InstitutionOK).To(BeTrue())
			Expect(section.KOSPINetNonArbitrageOrgan).To(BeZero())
		})
	})

	Context("when calculating late-session intervals", func() {
		var rows []map[string]any
		var section *LateSessionSection
		BeforeEach(func() {
			rows = []map[string]any{
				{"stck_bsop_date": "20260915", "bsop_hour": "150000", "whol_smtn_ntby_tr_pbmn": "100"},
				{"stck_bsop_date": "20260915", "bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "200"},
				{"stck_bsop_date": "20260915", "bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "500"},
			}
			section = &LateSessionSection{BusinessDate: "20260915"}
		})

		It("computes changes only from exact dated anchors", func() {
			Expect(fillLateProgramFlow(ctx, Deps{DomesticStock: fakeStock{compProg: response("output", rows...)}}, section, Options{})).To(Succeed())
			Expect(section.LateProgramNetEok).To(HaveValue(BeNumerically("~", 4, 1e-7)))
			Expect(section.CloseSessionProgramNetEok).To(HaveValue(BeNumerically("~", 3, 1e-7)))
		})

		DescribeTable("withholds changes when an anchor cannot be verified", func(mutate func([]map[string]any) []map[string]any) {
			rows = mutate(rows)
			Expect(fillLateProgramFlow(ctx, Deps{DomesticStock: fakeStock{compProg: response("output", rows...)}}, section, Options{})).To(Succeed())
			Expect(section.LateProgramNetEok).To(BeNil())
			Expect(section.CloseSessionProgramNetEok).To(BeNil())
		},
			Entry("dates are missing", func(rows []map[string]any) []map[string]any {
				for _, row := range rows {
					delete(row, "stck_bsop_date")
				}
				return rows
			}),
			Entry("the end sample is at 15:42", func(rows []map[string]any) []map[string]any { rows[2]["bsop_hour"] = "154200"; return rows }),
			Entry("the end sample is duplicated", func(rows []map[string]any) []map[string]any { return append(rows, rows[2]) }),
		)
	})

	Context("when selecting a 15:30 futures sample", func() {
		DescribeTable("rejects a first-row or nearby fallback", func(row map[string]any) {
			_, err := getFuturesPriceAtTime(ctx, fakeFuture{resp: response("output2", row)}, "101", "20260915", "153000")
			Expect(err).To(HaveOccurred())
		},
			Entry("15:45 on the requested date", map[string]any{"stck_bsop_date": "20260915", "stck_cntg_hour": "154500", "futs_prpr": "100"}),
			Entry("15:30 without a date", map[string]any{"stck_cntg_hour": "153000", "futs_prpr": "100"}),
			Entry("15:30 on the previous date", map[string]any{"stck_bsop_date": "20260914", "stck_cntg_hour": "153000", "futs_prpr": "100"}),
		)

		It("returns the price from an exact dated sample", func() {
			price, err := getFuturesPriceAtTime(ctx, fakeFuture{resp: response("output2", map[string]any{"stck_bsop_date": "20260915", "stck_cntg_hour": "153000", "futs_prpr": "101.25"})}, "101", "20260915", "153000")
			Expect(err).NotTo(HaveOccurred())
			Expect(price).To(BeNumerically("~", 101.25, 1e-7))
		})
	})

	Context("when reading index prices", func() {
		var row map[string]any
		BeforeEach(func() {
			row = map[string]any{"stck_bsop_date": "20260915", "bstp_nmix_prpr": "90", "bstp_nmix_oprc": "95", "bstp_nmix_hgpr": "98", "bstp_nmix_lwpr": "89", "bstp_nmix_prdy_vrss": "10", "prdy_vrss_sign": "5"}
		})

		It("applies the decline sign and preserves missing turnover as null", func() {
			price, ok := priceFromRow(row, "20260915")
			Expect(ok).To(BeTrue())
			Expect(price.PreviousClose).To(BeNumerically("~", 100, 1e-7))
			Expect(obj(price)).To(HaveKeyWithValue("trading_value_eok", BeNil()))
		})

		It("does not substitute a row from another date", func() {
			price, err := collectPrice(ctx, fakeStock{dailyRows: []map[string]any{row}}, "20260914")
			Expect(err).To(HaveOccurred())
			Expect(price).To(BeNil())
		})

		DescribeTable("rejects an invalid closing price", func(value string) {
			row["bstp_nmix_prpr"] = value
			_, ok := priceFromRow(row, "20260915")
			Expect(ok).To(BeFalse())
		}, Entry("NaN", "NaN"), Entry("infinity", "Inf"), Entry("zero", "0"), Entry("outside the daily range", "110"))
	})

	Context("when calculating concentration", func() {
		var cap *domesticstock.KOSPIMarketCapSummary
		BeforeEach(func() { cap = testCap() })

		It("uses the full universe denominator before selecting top constituents", func() {
			stock := indexStock{fakeStock{cap: cap}}
			concentration := collectConcentration(ctx, stock, "20260915")
			ratio, reason := autoSemiconductorCapRatio(ctx, stock, "20260915")
			Expect(reason).To(BeEmpty())
			Expect(concentration.Top10Percent).To(BeNumerically("~", 65.0/66*100, 1e-7))
			Expect(ratio).To(HaveValue(BeNumerically("~", 21.0/66*100, 1e-7)))
			Expect(concentration.Status).To(Equal(StatusEstimated))
			Expect(concentration.Top2ConcentrationStatus).NotTo(Equal("TOP2_CONCENTRATION_EXTREME"))
		})

		DescribeTable("withholds weights from an invalid universe", func(mutate func(*domesticstock.KOSPIMarketCapSummary)) {
			mutate(cap)
			stock := indexStock{fakeStock{cap: cap}}
			concentration := collectConcentration(ctx, stock, "20260915")
			ratio, _ := autoSemiconductorCapRatio(ctx, stock, "20260915")
			Expect(concentration.Status).To(Equal(StatusUnavailable))
			Expect(ratio).To(BeNil())
			Expect(obj(concentration)).To(HaveKeyWithValue("hhi", BeNil()))
		},
			Entry("a preferred share is included", func(c *domesticstock.KOSPIMarketCapSummary) { c.Constituents[0].PreferredClass = "1" }),
			Entry("a constituent is duplicated", func(c *domesticstock.KOSPIMarketCapSummary) { c.Constituents[0].Code = c.Constituents[1].Code }),
			Entry("the denominator omits a constituent", func(c *domesticstock.KOSPIMarketCapSummary) { c.TotalMarketCap = 65 }),
			Entry("the master date differs", func(c *domesticstock.KOSPIMarketCapSummary) { c.BusinessDate = "20260914" }),
			Entry("the source universe is mixed", func(c *domesticstock.KOSPIMarketCapSummary) { c.Universe = "MIXED" }),
			Entry("a market cap is nonfinite", func(c *domesticstock.KOSPIMarketCapSummary) { c.Constituents[0].MarketCap = math.NaN() }),
		)
	})

	Context("when computing the five-day VKOSPI average", func() {
		var stock historyStock
		var query string
		var now time.Time
		BeforeEach(func() {
			rows := []map[string]any{}
			for i, date := range []string{"20260915", "20260914", "20260911", "20260910", "20260909"} {
				rows = append(rows, map[string]any{"stck_bsop_date": date, "bstp_nmix_prpr": float64(40 + i)})
			}
			rows = append(rows, map[string]any{"stck_bsop_date": "20260916", "bstp_nmix_prpr": 99})
			query = ""
			stock = historyStock{fakeStock: fakeStock{vkospiDaily: rows}, queryDate: &query}
			now = time.Date(2026, 9, 15, 16, 0, 0, 0, kst.Location)
		})

		It("queries the report date and averages five dated rows without a future row or invented timestamp", func() {
			volatility := collectVolatility(ctx, stock, nil, nil, nil, 0, "20260915", Options{AsOf: now})
			Expect(query).To(Equal("20260915"))
			Expect(volatility.VKOSPI).To(BeNumerically("~", 40, 1e-7))
			Expect(volatility.VKOSPI5DayAvg).To(BeNumerically("~", 42, 1e-7))
			Expect(volatility.AverageDates).To(HaveLen(5))
			Expect(volatility.ObservedAt.IsZero()).To(BeTrue())
		})

		It("does not count a duplicated date toward a complete five-day average", func() {
			stock.vkospiDaily = append(stock.vkospiDaily, stock.vkospiDaily[0])
			volatility := collectVolatility(ctx, stock, nil, nil, nil, 0, "20260915", Options{AsOf: now})
			Expect(volatility.VKOSPI5DayAvg).To(BeZero())
		})
	})

	Context("when collecting lagged credit statistics", func() {
		It("keeps KIS and KOFIA reference dates separate while converting million KRW to eok", func() {
			stock := fundsStock{funds: captured("mktfunds")}
			source := fixedKofia{row: &kofia.MarketFundsRow{AmountUnit: "KRW_MILLION", Date: "20260914", MarginReceivableMln: 92444.38, ForcedSellAmountMln: 573.295, ForcedSellRatioPct: 0.6036}}
			credit, err := collectCredit(ctx, stock, source, "20260915")
			Expect(err).NotTo(HaveOccurred())
			Expect(credit.Date).To(Equal("20260911"))
			Expect(credit.ReferenceDate).To(Equal("20260911"))
			Expect(credit.KofiaDate).To(Equal("20260914"))
			Expect(credit.LagDays).To(Equal(4))
			Expect(credit.DepositReconciliationStatus).To(Equal(StatusValid))
			Expect(credit.MarginReceivableEok).To(BeNumerically("~", 924.4438, 1e-7))
			Expect(credit.ForcedSellAmountEok).To(BeNumerically("~", 5.73295, 1e-7))
		})

		It("flags a three-eok discrepancy instead of hiding it within a broad tolerance", func() {
			status, _ := ValidateCredit(1070560, 1076573, -6010)
			Expect(status).To(Equal(StatusArithmeticMismatch))
		})

		It("does not validate a missing balance", func() {
			status, _ := ValidateCredit(0, 1076573, -6013)
			Expect(status).NotTo(Equal(StatusValid))
		})
	})

	Context("when the requested session differs from collection time", func() {
		var now time.Time
		var deps Deps
		BeforeEach(func() {
			now = time.Date(2026, 9, 15, 10, 12, 0, 0, kst.Location)
			deps = Deps{Clock: func() time.Time { return now }, Yahoo: fakeYahoo{quotes: map[string]yahoo.Quote{"KRW=X": {Price: 1300, PreviousClose: 1290}}}}
		})

		It("keeps current-only data out of a historical report", func() {
			snapshot := Collect(ctx, deps, Options{Date: "20260914"})
			Expect(snapshot.Timestamp).To(Equal(now))
			Expect(snapshot.BusinessDate).To(Equal("20260914"))
			Expect(snapshot.Global).To(BeNil())
			Expect(snapshot.Macro).To(BeNil())
			Expect(snapshot.LateSession).To(BeNil())
			Expect(snapshot.Concentration).To(BeNil())
		})

		It("does not label an intraday collection as a completed close or expired eligibility", func() {
			snapshot := Collect(ctx, deps, Options{Date: "20260915"})
			Expect(snapshot.SessionStatus).To(Equal("INTRADAY_PROVISIONAL"))
			Expect(snapshot.Impact.EligibilityState).NotTo(Equal("EXPIRED_FOR_DAY"))
			Expect(Render(snapshot)).NotTo(ContainSubstring("15:300"))
		})

		It("rejects an invalid requested date", func() {
			Expect(Collect(ctx, deps, Options{Date: "nonsense"}).SessionStatus).To(Equal("INVALID_DATE"))
		})
	})

	Context("when model inputs are incomplete", func() {
		It("preserves unavailable correlation and risk scores as null", func() {
			regime := collectRegime(ctx, fakeYahoo{}, &PriceSection{Close: 900, PreviousClose: 1000}, &VolatilitySection{VKOSPI: 80}, nil, nil, nil)
			Expect(regime.DomesticMarketStressIdx).To(BeNil())
			Expect(obj(regime)).To(HaveKeyWithValue("global_risk_aversion_idx", BeNil()))
			Expect(obj(regime)).To(HaveKeyWithValue("kospi_nasdaq_corr", BeNil()))
		})

		It("does not identify a capitulation pattern from flows alone", func() {
			section := &LateSessionSection{LateProgramNetEok: ptr(-900), CloseSessionProgramNetEok: ptr(-900), CloseSessionForeignNetEok: ptr(-900), CloseSessionOrganNetEok: ptr(-900)}
			evaluateLateSessionPatterns(&PriceSection{Low: 80, High: 100, Close: 80}, section)
			Expect(section.PatternEvaluated).To(BeFalse())
			Expect(section.CapitulationScore).To(BeNil())
		})
	})

	Context("when exporting and replacing a snapshot", func() {
		var snapshot *Snapshot
		BeforeEach(func() {
			now := time.Date(2026, 9, 15, 16, 2, 3, 0, kst.Location)
			snapshot = &Snapshot{BusinessDate: "20260914", Timestamp: now, SessionStatus: "HISTORICAL_DATED_DATA_ONLY", Impact: &ImpactSection{}, Cumulative: &CumulativeSection{}, Global: &GlobalSection{Quotes: map[string]yahoo.Quote{"X": {Price: 100, MarketTimeUnix: now.Unix()}}}}
		})

		It("preserves the business date, sections, quote timestamp, and missing change", func() {
			output := obj(snapshot.ToJSON())
			Expect(output).To(HaveKeyWithValue("date", "20260914"))
			Expect(output).To(HaveKeyWithValue("impact", Not(BeNil())))
			Expect(output).To(HaveKeyWithValue("cumulative", Not(BeNil())))
			quotes := output["global"].(map[string]any)["quotes"].(map[string]any)
			Expect(quotes["X"]).To(HaveKeyWithValue("change_percent", BeNil()))
			Expect(quotes["X"]).To(HaveKeyWithValue("market_time_unix", Not(BeNil())))
		})

		It("archives the original bytes before replacing the dated file", func() {
			dir := GinkgoT().TempDir()
			path, err := SaveJSON(snapshot, dir)
			Expect(err).NotTo(HaveOccurred())
			original, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			snapshot.Impact.SidecarStatus = "unknown"
			_, err = SaveJSON(snapshot, dir)
			Expect(err).NotTo(HaveOccurred())
			archives, err := filepath.Glob(filepath.Join(dir, "*.previous.*.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(archives).To(HaveLen(1))
			archived, err := os.ReadFile(archives[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(archived).To(Equal(original))
		})
	})
})
