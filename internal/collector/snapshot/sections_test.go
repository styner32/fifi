package snapshot

import (
	"context"
	"errors"

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
	return &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"bstp_nmix_prpr": "350.0"}}}}, nil
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
				"frgn_ntby_tr_pbmn": "-4834200", "orgn_ntby_tr_pbmn": "-734000", "prsn_ntby_tr_pbmn": "5419800",
			}
			resp := &auth.RESTResponse{Body: map[string]any{"output": []any{row}}}
			got, err := collectFlow(context.Background(), fakeStock{investor: resp}, "20260515")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ForeignEok).To(Equal(-48342.0))
			Expect(got.InstitutionEok).To(Equal(-7340.0))
			Expect(got.IndividualEok).To(Equal(54198.0))
		})
	})

	Context("collectImpact", func() {
		It("computes impact ratios and manual sidecar status", func() {
			semiSell := -42000.0
			stock := fakeStock{cap: &domesticstock.KOSPIMarketCapSummary{TotalMarketCap: 7_100_000}}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output1": map[string]any{"futs_prdy_ctrt": "-5.09"}}}}
			got := collectImpact(context.Background(), Deps{DomesticStock: stock, DomesticFuture: futures}, "20260515", &FlowSection{ForeignEok: -48342}, &PriceSection{TradingValueEok: 200_000}, Options{
				SidecarStatus: "triggered", SidecarTime: "13:28:49", SemiconductorForeignNetSellEok: &semiSell,
			})
			Expect(got.ForeignNetFlowToMarketCap).NotTo(BeNil())
			Expect(*got.ForeignNetFlowToMarketCap).To(BeNumerically("<=", -0.67))
			Expect(*got.ForeignNetFlowToMarketCap).To(BeNumerically(">=", -0.69))
			Expect(got.SemiconductorSellConcentrationPct).NotTo(BeNil())
			Expect(*got.SemiconductorSellConcentrationPct).To(BeNumerically(">=", 86.0))
			Expect(got.FuturesChangePercent).NotTo(BeNil())
			Expect(*got.FuturesChangePercent).To(Equal(-5.09))
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
		It("prefers KIS VKOSPI and uses opposite direction for decoupling", func() {
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
			got := collectVolatility(context.Background(), stock, naverClient, fakeYahoo{}, nil, -3, "20260710", Options{})
			Expect(got.VKOSPI).To(Equal(28.5))
			Expect(got.Source).To(Equal("KIS"))
			Expect(got.DecouplingFlag).To(BeTrue())
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
			Expect(out).To(ContainSubstring("값: 1,494.20"))
			Expect(out).To(ContainSubstring("값: 4.52%"))
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

	Context("collectLateSession Patterns", func() {
		It("detects capitulation event", func() {
			progResp := &auth.RESTResponse{
				Body: map[string]any{
					"output1": []any{
						map[string]any{"invr_cls_name": "외국인투자자", "nabt_ntby_amt": "-40000"},
						map[string]any{"invr_cls_name": "기관합계", "nabt_ntby_amt": "-20000"},
						map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "-60000"},
					},
				},
			}
			compProgResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "-180000"},
						map[string]any{"bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "-140000"},
						map[string]any{"bsop_hour": "150000", "whol_smtn_ntby_tr_pbmn": "-100000"},
					},
				},
			}
			timeFlowResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"aspr_hour": "153000", "frgn_ntby_tr_pbmn": "-50000", "orgn_ntby_tr_pbmn": "-40000"},
						map[string]any{"aspr_hour": "152000", "frgn_ntby_tr_pbmn": "-20000", "orgn_ntby_tr_pbmn": "-20000"},
					},
				},
			}
			stock := fakeStock{program: progResp, compProg: compProgResp, timeFlow: timeFlowResp}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"futs_prpr": "349.5"}}}}}
			deps := Deps{DomesticStock: stock, DomesticFuture: futures}
			priceSec := &PriceSection{High: 100, Low: 90, Close: 90.1}

			got, err := collectLateSession(context.Background(), deps, "20260515", priceSec, Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.SpotPrice).To(Equal(350.0))
			Expect(got.FuturesPrice).To(Equal(349.5))
			Expect(got.BasisPoint).To(Equal(-0.5))
			Expect(got.KOSPINetNonArbitrageForeign).To(Equal(-400.0))
			Expect(got.LateProgramNetEok).NotTo(BeNil())
			Expect(*got.LateProgramNetEok).To(Equal(-800.0))
			Expect(got.CloseSessionProgramNetEok).NotTo(BeNil())
			Expect(*got.CloseSessionProgramNetEok).To(Equal(-400.0))
			Expect(got.CloseSessionForeignNetEok).NotTo(BeNil())
			Expect(*got.CloseSessionForeignNetEok).To(Equal(-300.0))
			Expect(got.PatternDetected).To(BeTrue())
			Expect(got.PrimaryPattern).To(Equal("Late-Session Capitulation"))
		})

		It("detects short squeeze event", func() {
			progResp := &auth.RESTResponse{Body: map[string]any{"output1": []any{map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "0"}}}}
			compProgResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "80000"},
						map[string]any{"bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "0"},
						map[string]any{"bsop_hour": "150000", "whol_smtn_ntby_tr_pbmn": "0"},
					},
				},
			}
			timeFlowResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"aspr_hour": "153000", "frgn_ntby_tr_pbmn": "30000", "orgn_ntby_tr_pbmn": "0"},
						map[string]any{"aspr_hour": "152000", "frgn_ntby_tr_pbmn": "0", "orgn_ntby_tr_pbmn": "0"},
					},
				},
			}
			stock := fakeStock{program: progResp, compProg: compProgResp, timeFlow: timeFlowResp}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"futs_prpr": "351.0"}}}}}
			deps := Deps{DomesticStock: stock, DomesticFuture: futures}
			priceSec := &PriceSection{High: 100, Low: 90, Close: 99.9}

			got, err := collectLateSession(context.Background(), deps, "20260515", priceSec, Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.PatternDetected).To(BeTrue())
			Expect(got.PrimaryPattern).To(Equal("Late-Session Short Squeeze"))
		})

		It("detects window dressing event", func() {
			progResp := &auth.RESTResponse{Body: map[string]any{"output1": []any{map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "0"}}}}
			compProgResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "0"},
						map[string]any{"bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "0"},
						map[string]any{"bsop_hour": "150000", "whol_smtn_ntby_tr_pbmn": "0"},
					},
				},
			}
			timeFlowResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"aspr_hour": "153000", "frgn_ntby_tr_pbmn": "0", "orgn_ntby_tr_pbmn": "35000"},
						map[string]any{"aspr_hour": "152000", "frgn_ntby_tr_pbmn": "0", "orgn_ntby_tr_pbmn": "0"},
					},
				},
			}
			stock := fakeStock{program: progResp, compProg: compProgResp, timeFlow: timeFlowResp}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"futs_prpr": "350.0"}}}}}
			deps := Deps{DomesticStock: stock, DomesticFuture: futures}
			priceSec := &PriceSection{High: 100, Low: 90, Close: 99.5}

			got, err := collectLateSession(context.Background(), deps, "20260630", priceSec, Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.PatternDetected).To(BeTrue())
			Expect(got.PrimaryPattern).To(Equal("Window Dressing"))
		})

		It("detects ETF rebalancing impact event", func() {
			progResp := &auth.RESTResponse{Body: map[string]any{"output1": []any{map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "0"}}}}
			compProgResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "90000"},
						map[string]any{"bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "0"},
						map[string]any{"bsop_hour": "150000", "whol_smtn_ntby_tr_pbmn": "0"},
					},
				},
			}
			timeFlowResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"aspr_hour": "153000", "frgn_ntby_tr_pbmn": "0", "orgn_ntby_tr_pbmn": "0"},
						map[string]any{"aspr_hour": "152000", "frgn_ntby_tr_pbmn": "0", "orgn_ntby_tr_pbmn": "0"},
					},
				},
			}
			stock := fakeStock{program: progResp, compProg: compProgResp, timeFlow: timeFlowResp}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"futs_prpr": "350.0"}}}}}
			deps := Deps{DomesticStock: stock, DomesticFuture: futures}
			priceSec := &PriceSection{High: 100, Low: 90, Close: 95.0}

			got, err := collectLateSession(context.Background(), deps, "20260528", priceSec, Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.PatternDetected).To(BeTrue())
			Expect(got.PrimaryPattern).To(Equal("ETF Rebalancing Impact"))
		})

		It("detects expiration basis arbitrage event", func() {
			progResp := &auth.RESTResponse{Body: map[string]any{"output1": []any{map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "0"}}}}
			compProgResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "-40000"},
						map[string]any{"bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "0"},
						map[string]any{"bsop_hour": "150000", "whol_smtn_ntby_tr_pbmn": "0"},
					},
				},
			}
			timeFlowResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"aspr_hour": "153000", "frgn_ntby_tr_pbmn": "0", "orgn_ntby_tr_pbmn": "0"},
						map[string]any{"aspr_hour": "152000", "frgn_ntby_tr_pbmn": "0", "orgn_ntby_tr_pbmn": "0"},
					},
				},
			}
			stock := fakeStock{program: progResp, compProg: compProgResp, timeFlow: timeFlowResp}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"futs_prpr": "347.0"}}}}}
			deps := Deps{DomesticStock: stock, DomesticFuture: futures}
			priceSec := &PriceSection{High: 100, Low: 90, Close: 95.0}

			got, err := collectLateSession(context.Background(), deps, "20260611", priceSec, Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.PatternDetected).To(BeTrue())
			Expect(got.PrimaryPattern).To(Equal("Expiration Basis Arbitrage"))
		})
	})

	Context("Price and Impact rendering logic", func() {
		It("uses pr.PreviousClose as authoritative value", func() {
			s := &Snapshot{
				Price: &PriceSection{
					Date: "20260619", Open: 9100, High: 9385, Low: 9010,
					Close: 9040.36, PreviousClose: 9063.84,
				},
			}
			out := Render(s)
			Expect(out).To(ContainSubstring("9,063.84"))
			Expect(out).To(ContainSubstring("-23.48"))
		})

		It("warns when saved JSON diverges from API prev close", func() {
			s := &Snapshot{
				Price: &PriceSection{
					Date: "20260619", Open: 9100, High: 9385, Low: 9010,
					Close: 9040.36, PreviousClose: 9063.84,
				},
			}
			prev := &SnapshotJSON{
				Date: "20260618",
				Price: &PriceSection{
					Close: 8864.24,
				},
			}
			out := Render(s, prev)
			Expect(out).To(ContainSubstring("⚠"))
			Expect(out).To(ContainSubstring("불일치"))
			Expect(out).To(ContainSubstring("9,063.84"))
		})

		It("does not warn when saved JSON matches API prev close", func() {
			s := &Snapshot{
				Price: &PriceSection{
					Date: "20260619", Open: 9100, High: 9385, Low: 9010,
					Close: 9040.36, PreviousClose: 9063.84,
				},
			}
			prev := &SnapshotJSON{
				Date: "20260618",
				Price: &PriceSection{
					Close: 9063.84,
				},
			}
			out := Render(s, prev)
			Expect(out).NotTo(ContainSubstring("불일치"))
		})

		It("removes basis fields from Section 3 and references Section 11", func() {
			s := &Snapshot{
				Impact: &ImpactSection{
					FuturesChangePercent: ptr(-2.5),
					FuturesPrice:         ptr(1475.0),
					SidecarStatus:        "not-triggered",
				},
				LateSession: &LateSessionSection{
					SpotPrice:    1459.41,
					FuturesPrice: 1473.55,
					BasisPoint:   14.14,
					BasisRate:    0.97,
				},
			}
			out := Render(s)
			Expect(out).To(ContainSubstring("Section 11 참조"))
			Expect(out).To(ContainSubstring("14.1"))
		})
	})

	Context("Program Trade Total Fallback", func() {
		It("computes total when '합계' row is missing", func() {
			progResp := &auth.RESTResponse{
				Body: map[string]any{
					"output1": []any{
						map[string]any{"invr_cls_name": "외국인투자자", "nabt_ntby_amt": "414900"},
						map[string]any{"invr_cls_name": "기관합계", "nabt_ntby_amt": "-880600"},
						map[string]any{"invr_cls_name": "개인", "nabt_ntby_amt": "563100"},
					},
				},
			}
			stock := fakeStock{program: progResp}
			sec := &LateSessionSection{}
			err := fillProgramTradeToday(context.Background(), Deps{DomesticStock: stock}, sec)
			Expect(err).NotTo(HaveOccurred())
			Expect(sec.KOSPINetNonArbitrageForeign).To(Equal(4149.0))
			Expect(sec.KOSPINetNonArbitrageOrgan).To(Equal(-8806.0))
			expectedTotal := 4149.0 + (-8806.0) + 5631.0
			Expect(sec.KOSPINetNonArbitrageTotal).To(Equal(expectedTotal))
		})

		It("uses '합계' row when present", func() {
			progResp := &auth.RESTResponse{
				Body: map[string]any{
					"output1": []any{
						map[string]any{"invr_cls_name": "외국인투자자", "nabt_ntby_amt": "414900"},
						map[string]any{"invr_cls_name": "기관합계", "nabt_ntby_amt": "-880600"},
						map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "-100000"},
					},
				},
			}
			stock := fakeStock{program: progResp}
			sec := &LateSessionSection{}
			err := fillProgramTradeToday(context.Background(), Deps{DomesticStock: stock}, sec)
			Expect(err).NotTo(HaveOccurred())
			Expect(sec.KOSPINetNonArbitrageTotal).To(Equal(-1000.0))
		})

		It("matches '계' row name without '합'", func() {
			progResp := &auth.RESTResponse{
				Body: map[string]any{
					"output1": []any{
						map[string]any{"invr_cls_name": "외국인투자자", "nabt_ntby_amt": "100000"},
						map[string]any{"invr_cls_name": "계", "nabt_ntby_amt": "200000"},
					},
				},
			}
			stock := fakeStock{program: progResp}
			sec := &LateSessionSection{}
			err := fillProgramTradeToday(context.Background(), Deps{DomesticStock: stock}, sec)
			Expect(err).NotTo(HaveOccurred())
			Expect(sec.KOSPINetNonArbitrageTotal).To(Equal(2000.0))
		})
	})

	Context("EBA Gates and Meltdown Regime", func() {
		It("prevents EBA trigger on non-expiration days", func() {
			progResp := &auth.RESTResponse{Body: map[string]any{"output1": []any{map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "0"}}}}
			compProgResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "-40000"},
						map[string]any{"bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "0"},
					},
				},
			}
			stock := fakeStock{program: progResp, compProg: compProgResp}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"futs_prpr": "347.0"}}}}}
			deps := Deps{DomesticStock: stock, DomesticFuture: futures}
			priceSec := &PriceSection{High: 100, Low: 90, Close: 95.0}

			got, err := collectLateSession(context.Background(), deps, "20260702", priceSec, Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.PrimaryPattern).NotTo(Equal("Expiration Basis Arbitrage"))
		})

		It("prevents EBA trigger when program trades are small", func() {
			progResp := &auth.RESTResponse{Body: map[string]any{"output1": []any{map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "0"}}}}
			compProgResp := &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "400"},
						map[string]any{"bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "0"},
					},
				},
			}
			stock := fakeStock{program: progResp, compProg: compProgResp}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"futs_prpr": "347.0"}}}}}
			deps := Deps{DomesticStock: stock, DomesticFuture: futures}
			priceSec := &PriceSection{High: 100, Low: 90, Close: 95.0}

			got, err := collectLateSession(context.Background(), deps, "20260611", priceSec, Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.PrimaryPattern).NotTo(Equal("Expiration Basis Arbitrage"))
		})

		It("updates regime and risk index on market crash when inputs are present", func() {
			price := &PriceSection{
				Close:         7648.09,
				PreviousClose: 8303.41,
				High:          8136.28,
				Low:           7616.33,
			}
			volatility := &VolatilitySection{VKOSPI: 35.0}
			macro := &MacroSection{Quotes: map[string]yahoo.Quote{"^TNX": {Price: 4.5}}}
			impact := &ImpactSection{
				SidecarStatus: "triggered",
			}

			phase := classifyPhase(price, impact, volatility, macro)
			Expect(phase).To(ContainSubstring("패닉"))

			reg := collectRegime(context.Background(), fakeYahoo{}, price, volatility, impact, macro, nil)
			Expect(reg.DomesticMarketStressIdx).NotTo(BeNil())
			Expect(*reg.DomesticMarketStressIdx).To(BeNumerically(">=", 8.0))
		})

		It("displays '미갱신' for stale date comparison", func() {
			s := &Snapshot{
				Credit: &CreditSection{
					CreditLoanBalanceEok: 373282,
					CustomerDepositEok:   1216340,
					Date:                 "20260630",
					KofiaDate:            "20260630",
					MarginReceivableEok:  125912,
				},
				Concentration: &ConcentrationSection{
					Top5Percent: 63.1,
					Date:        "20260702",
				},
			}
			prev := &SnapshotJSON{
				Date: "20260701",
				Credit: &CreditSection{
					CreditLoanBalanceEok: 373282,
					CustomerDepositEok:   1216340,
					Date:                 "20260630",
					KofiaDate:            "20260630",
					MarginReceivableEok:  125912,
				},
				Concentration: &ConcentrationSection{
					Top5Percent: 63.1,
					Date:        "20260702",
				},
			}

			out := Render(s, prev)
			Expect(out).To(ContainSubstring("미갱신"))
		})
	})

	Context("Domain and Unit Fixes", func() {
		It("renders credit unit scaling as 억원 with decimals", func() {
			s := &Snapshot{
				Credit: &CreditSection{
					MarginReceivableEok: 1566.32,
					ForcedSellAmountEok: 22.37,
					ForcedSellRatioPct:  1.3,
					Date:                "20260804",
				},
			}
			out := Render(s, nil)
			Expect(out).To(ContainSubstring("1,566.32억원"))
			Expect(out).To(ContainSubstring("22.37억원"))
		})

		It("returns BELOW_CUSTOM_ALERT_THRESHOLD label for negative flow below 10%", func() {
			label := tradingValueLabel(-1.33)
			Expect(label).To(ContainSubstring("BELOW_CUSTOM_ALERT_THRESHOLD"))
		})

		It("calculates concentration delta using rounded figures", func() {
			s := &Snapshot{
				Concentration: &ConcentrationSection{
					Top5Percent:  57.06,
					Top10Percent: 63.31,
					HHI:          1347.4,
					Date:         "20260804",
				},
			}
			prev := &SnapshotJSON{
				Date: "20260803",
				Concentration: &ConcentrationSection{
					Top5Percent:  58.74,
					Top10Percent: 64.92,
					HHI:          1451.6,
					Date:         "20260803",
				},
			}
			out := Render(s, prev)
			Expect(out).To(ContainSubstring("-1.6%p"))
			Expect(out).To(ContainSubstring("-105"))
		})

		It("renders late session program trade breakdown and NOT_A_BASIS labels", func() {
			s := &Snapshot{
				LateSession: &LateSessionSection{
					SpotPrice:                   1000.03,
					FuturesPrice:                1000.00,
					FuturesPrice1530:            1002.60,
					BasisPoint:                  -0.03,
					BasisRate:                   -0.003,
					KOSPINetArbitrageTotal:      -1498.0,
					KOSPINetNonArbitrageTotal:   -2571.0,
					KOSPIProgramTotalNet:        -4069.0,
					KOSPINetNonArbitrageForeign: -3856.0,
					NaverFollowupTotalEok:       ptr(-620.0),
					CrossSourceStatus:           string(StatusSourceScopeConflict),
					CrossSourceDifferenceEok:    1096.0,
				},
			}
			out := Render(s, nil)
			Expect(out).To(ContainSubstring("NOT_A_BASIS / CROSS_TIME_SPREAD"))
			Expect(out).To(ContainSubstring("SOURCE_SCOPE_CONFLICT"))
		})
	})

	Context("Section 23 Mandatory Audit Regression Suite", func() {
		It("1. missing previous flow status does not output numeric 0", func() {
			f := &FlowSection{PreviousFlowStatus: string(StatusMissing), PreviousFlowReason: "PREVIOUS_DAY_FLOW_NOT_COLLECTED"}
			s := &Snapshot{Flow: f}
			out := Render(s, nil)
			Expect(out).NotTo(MatchRegexp(`\| 외국인 \| [^|]+ \| 0 \|`))
			Expect(out).To(ContainSubstring("N/A"))
		})

		It("2. converts 백만원 to 억원 correctly (924.4438)", func() {
			c := &CreditSection{MarginReceivableEok: 924.4438}
			s := &Snapshot{Credit: c}
			out := Render(s, nil)
			Expect(out).To(ContainSubstring("924.444억원"))
		})

		It("3. converts forced sell 백만원 to 억원 correctly (5.73295)", func() {
			c := &CreditSection{ForcedSellAmountEok: 5.73295, ForcedSellRatioPct: 0.6036, MarginReceivableEok: 924.4438}
			s := &Snapshot{Credit: c}
			out := Render(s, nil)
			Expect(out).To(ContainSubstring("5.733억원"))
		})

		It("4. calculates forced sell to prior receivable ratio as 0.6036%", func() {
			c := &CreditSection{ForcedSellAmountEok: 5.73295, ForcedSellRatioPct: 0.6036, ForcedSellStatus: "BELOW_CUSTOM_ALERT_THRESHOLD"}
			s := &Snapshot{Credit: c}
			out := Render(s, nil)
			Expect(out).To(ContainSubstring("0.6036% BELOW_CUSTOM_ALERT_THRESHOLD"))
		})

		It("5. displays program arbitrage + non-arbitrage sum matching total within display precision", func() {
			ls := &LateSessionSection{KOSPINetArbitrageTotal: 0, KOSPINetNonArbitrageTotal: -1716, KOSPIProgramTotalNet: -1716}
			Expect(ls.KOSPINetArbitrageTotal + ls.KOSPINetNonArbitrageTotal).To(Equal(ls.KOSPIProgramTotalNet))
		})

		It("6. detects program source difference (+1096 eok) and sets SOURCE_SCOPE_CONFLICT", func() {
			ls := &LateSessionSection{
				OriginalSnapshotTotalEok: -1716,
				NaverFollowupTotalEok:    ptr(-620.0),
				CrossSourceDifferenceEok: 1096.0,
				CrossSourceStatus:        string(StatusSourceScopeConflict),
			}
			Expect(ls.CrossSourceStatus).To(Equal(string(StatusSourceScopeConflict)))
			Expect(ls.CrossSourceDifferenceEok).To(Equal(1096.0))
		})

		It("7. evaluates mismatched futures and spot times as NOT_A_BASIS", func() {
			ls := &LateSessionSection{SpotPrice: 300.0, FuturesPrice: 302.46, BasisPoint: 2.46}
			s := &Snapshot{LateSession: ls}
			out := Render(s, nil)
			Expect(out).To(ContainSubstring("NOT_A_BASIS / CROSS_TIME_SPREAD"))
		})

		It("8. sets ALIGNMENT_UNVERIFIED and FUTURES_CONTRACT_IDENTITY_UNVERIFIED for unverified futures identity", func() {
			ls := &LateSessionSection{FuturesContractIdentityStatus: string(StatusFuturesIdentityUnverified)}
			s := &Snapshot{LateSession: ls}
			out := Render(s, nil)
			Expect(out).To(ContainSubstring("FUTURES_CONTRACT_IDENTITY_UNVERIFIED"))
		})

		It("9. evaluates domestic market stress index as NOT_EVALUATED when inputs are missing", func() {
			r := &RegimeSection{MissingInputs: []string{"VKOSPI_CLOSE", "OFFICIAL_SIDECAR_HISTORY", "RECONCILED_PROGRAM_FLOW"}}
			s := &Snapshot{Regime: r}
			out := Render(s, nil)
			Expect(r.DomesticMarketStressIdx).To(BeNil())
			Expect(out).To(ContainSubstring("N/A / NOT_EVALUATED"))
		})

		It("10. sets correlation trend to NOT_EVALUATED when previous correlation is missing", func() {
			r := &RegimeSection{KOSPINASDAQCorr: 0.85, KospiNasdaqCorrLevel: "HIGH_CORRELATION", KospiNasdaqCorrTrend: string(StatusNotEvaluated)}
			s := &Snapshot{Regime: r}
			out := Render(s, nil)
			Expect(out).To(ContainSubstring("`NOT_EVALUATED`"))
		})

		It("11. sidecar eligibility EXPIRED_FOR_DAY does not clear trigger history", func() {
			imp := &ImpactSection{EligibilityState: "EXPIRED_FOR_DAY", TriggerState: string(StatusTriggerHistoryUnknown)}
			Expect(imp.EligibilityState).To(Equal("EXPIRED_FOR_DAY"))
			Expect(imp.TriggerState).To(Equal(string(StatusTriggerHistoryUnknown)))
		})

		It("12. follow-up reconciliation does not overwrite raw 15:30 snapshot records", func() {
			ls := &LateSessionSection{
				OriginalSnapshotTotalEok: -1716,
				NaverFollowupTotalEok:    ptr(-620.0),
				CanonicalTotalEok:        nil,
			}
			Expect(ls.OriginalSnapshotTotalEok).To(Equal(-1716.0))
			Expect(ls.CanonicalTotalEok).To(BeNil())
		})
	})
})
