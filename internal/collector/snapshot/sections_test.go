package snapshot

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/naver"
	"github.com/fifi/internal/external/yahoo"
)

type fakeStock struct {
	dailyRows   []map[string]any
	investor    *auth.RESTResponse
	prices      map[string]*auth.RESTResponse
	cap         *domesticstock.KOSPIMarketCapSummary
	program     *auth.RESTResponse
	timeFlow    *auth.RESTResponse
	compProg    *auth.RESTResponse
	vkospi      *auth.RESTResponse
	vkospiDaily []map[string]any
}

func (f fakeStock) InquireIndexDailyPrice(context.Context, string, string) ([]map[string]any, error) {
	return f.dailyRows, nil
}
func (f fakeStock) InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error) {
	return &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output": []any{map[string]any{"bstp_nmix_prpr": "350.0"}}}}, nil
}
func (f fakeStock) InquireInvestorDailyByMarket(context.Context, string) (*auth.RESTResponse, error) {
	return f.investor, nil
}
func (f fakeStock) InquirePrice(_ context.Context, symbol string) (*auth.RESTResponse, error) {
	if resp, ok := f.prices[symbol]; ok {
		return resp, nil
	}
	return nil, errors.New("price missing: " + symbol)
}
func (f fakeStock) KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error) {
	if f.cap == nil {
		return nil, errors.New("cap summary missing")
	}
	return f.cap, nil
}
func (f fakeStock) ResolveVKOSPICode(context.Context, []string) (string, error) {
	return "2050", nil
}
func (f fakeStock) InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error) {
	return f.vkospi, nil
}
func (f fakeStock) InquireVKOSPIDailyPrice(context.Context, string, string) ([]map[string]any, error) {
	return f.vkospiDaily, nil
}
func (f fakeStock) MarketFunds(context.Context, string) (*auth.RESTResponse, error) {
	return nil, nil
}
func (f fakeStock) InvestorProgramTradeToday(context.Context, string) (*auth.RESTResponse, error) {
	return f.program, nil
}
func (f fakeStock) InquireInvestorTimeByMarket(context.Context, string, string) (*auth.RESTResponse, error) {
	return f.timeFlow, nil
}
func (f fakeStock) CompProgramTradeToday(context.Context, string) (*auth.RESTResponse, error) {
	return f.compProg, nil
}

type fakeNaver struct {
	quote   *naver.IndexQuote
	history []naver.DailyClose
	err     error
}

func (f fakeNaver) GetIndexQuote(context.Context, string) (*naver.IndexQuote, error) {
	return f.quote, f.err
}
func (f fakeNaver) GetIndexDailyHistory(context.Context, string, int) ([]naver.DailyClose, error) {
	return f.history, f.err
}

type fakeFuture struct {
	resp *auth.RESTResponse
}

func (f fakeFuture) ResolveNearMonthKOSPI200Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error) {
	return &domesticfutureoption.ResolvedContract{Record: domesticfutureoption.MasterRecord{ShortCode: "101V03"}}, nil
}
func (f fakeFuture) InquirePrice(context.Context, string, string) (*auth.RESTResponse, error) {
	return f.resp, nil
}
func (f fakeFuture) InquireTimeFuopChartPrice(ctx context.Context, marketDivCode, inputISCD, hourClsCode, includePastData, includeFakeTick, inputDate, inputHour string) (*auth.RESTResponse, error) {
	return f.resp, nil
}

type fakeYahoo struct {
	quotes map[string]yahoo.Quote
	err    error
}

func (f fakeYahoo) GetQuotes(context.Context, []string) (map[string]yahoo.Quote, error) {
	return f.quotes, f.err
}
func (f fakeYahoo) GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error) {
	return nil, nil
}

var _ = Describe("Snapshot Collector Sections", func() {
	Context("collectPrice", func() {
		It("collects daily price range and year high", func() {
			rows := []map[string]any{{
				"stck_bsop_date": "20260515", "bstp_nmix_prpr": "7950",
				"bstp_nmix_oprc": "7900", "bstp_nmix_hgpr": "8050", "bstp_nmix_lwpr": "7600",
				"stck_prdy_clpr": "8000", "dryy_bstp_nmix_hgpr": "8050",
			}}
			got, err := collectPrice(context.Background(), fakeStock{dailyRows: rows}, "20260515")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.RangePoints).To(Equal(450.0))
			Expect(got.RangePercent).To(Equal(5.625))
			Expect(got.YearHigh).To(BeTrue())
		})
	})

	Context("collectFlow", func() {
		It("converts trade amounts from million KRW to eok", func() {
			row := map[string]any{
				"stck_bsop_date": "20260515", "frgn_ntby_tr_pbmn": "-4834200", "orgn_ntby_tr_pbmn": "-734000", "prsn_ntby_tr_pbmn": "5419800",
			}
			resp := &auth.RESTResponse{Body: map[string]any{"rt_cd": "0", "output": []any{row}}}
			got, err := collectFlow(context.Background(), fakeStock{investor: resp}, "20260515")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ForeignEok).To(Equal(-48342.0))
			Expect(got.InstitutionEok).To(Equal(-7340.0))
			Expect(got.IndividualEok).To(Equal(54198.0))
		})
	})

	Context("collectImpact", func() {
		It("withholds unreconciled ratios and historical current quotes", func() {
			semiSell := -42000.0
			stock := fakeStock{cap: &domesticstock.KOSPIMarketCapSummary{TotalMarketCap: 7_100_000}}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output1": map[string]any{"futs_prdy_ctrt": "-5.09"}}}}
			got := collectImpact(context.Background(), Deps{DomesticStock: stock, DomesticFuture: futures}, "20260515", &FlowSection{ForeignEok: -48342}, &PriceSection{TradingValueEok: 200_000}, Options{
				SidecarStatus: "triggered", SidecarTime: "13:28:49", SemiconductorForeignNetSellEok: &semiSell,
			})
			Expect(got.ForeignNetFlowToMarketCap).To(BeNil())
			Expect(got.SemiconductorSellConcentrationPct).NotTo(BeNil())
			Expect(*got.SemiconductorSellConcentrationPct).To(BeNumerically(">=", 86.0))
			Expect(got.FuturesChangePercent).To(BeNil())
			Expect(got.SidecarStatus).To(Equal("triggered"))
		})
	})

	Context("collectGlobal", func() {
		It("keeps partial quotes when some fail", func() {
			quotes := map[string]yahoo.Quote{
				"KRW=X": {Symbol: "KRW=X", Price: 1494.2, ChangePercent: 0.51},
			}
			err := errors.New("yahoo quote missing: BTC-USD")
			got, gErr := collectGlobal(context.Background(), fakeYahoo{quotes: quotes, err: err})
			Expect(gErr).NotTo(HaveOccurred())
			Expect(got.Reason).To(ContainSubstring("BTC-USD"))
		})
	})

	Context("collectVolatility", func() {
		It("retains undated KIS VKOSPI as unverified and does not infer decoupling", func() {
			stock := fakeStock{
				vkospi: &auth.RESTResponse{Body: map[string]any{
					"rt_cd": "0",
					"output": map[string]any{
						"bstp_nmix_prpr": "28.50", "bstp_nmix_prdy_ctrt": "12.30",
					},
				}},
				vkospiDaily: []map[string]any{
					{"bstp_nmix_prpr": "28.50"}, {"bstp_nmix_prpr": "25.00"},
					{"bstp_nmix_prpr": "24.00"}, {"bstp_nmix_prpr": "23.00"}, {"bstp_nmix_prpr": "22.00"},
				},
			}
			naverClient := fakeNaver{quote: &naver.IndexQuote{Price: 99, ChangePercent: -20}}
			got := collectVolatility(context.Background(), stock, naverClient, fakeYahoo{}, nil, -3, "20260710", Options{AsOf: time.Date(2026, 7, 10, 14, 0, 0, 0, time.FixedZone("KST", 32400))})
			Expect(got.VKOSPI).To(Equal(28.5))
			Expect(got.Source).To(Equal("KIS"))
			Expect(got.DecouplingFlag).To(BeFalse())
			Expect(got.Status).To(Equal(StatusTimestampMissing))
			Expect(got.VKOSPI5DayAvg).To(BeZero())
			Expect(isDecoupling(-3, -12.3)).To(BeFalse())
		})
	})

	Context("collectMacro", func() {
		It("renders USD/KRW month start and TNX scale", func() {
			start := 1430.0
			s := &Snapshot{Macro: &MacroSection{USDKRWMonthStart: &start, USDKRWMonthStartPct: ptr(4.49), Quotes: map[string]yahoo.Quote{
				"KRW=X": {Price: 1494.2}, "CL=F": {Price: 102.34}, "^TNX": {Price: 4.52},
			}}}
			out := Render(s)
			Expect(out).To(ContainSubstring("1,494.20 KRW/USD"))
			Expect(out).To(ContainSubstring("4.52%"))
		})
	})

	Context("collectCumulative", func() {
		It("handles manual monthly values and missing values", func() {
			monthly := -202000.0
			got := collectCumulative(context.Background(), nil, "20260515", Options{MonthlyForeignNetSellEok: &monthly})
			Expect(got.MonthlyForeignNetSellEok).NotTo(BeNil())
			Expect(got.ForeignHoldingReason).NotTo(BeEmpty())
			Expect(got.CapRatioReason).NotTo(BeEmpty())
		})
	})

})
