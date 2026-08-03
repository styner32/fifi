package snapshot

import (
	"context"
	"errors"
	"strings"
	"testing"

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
	// KOSPI 200 지수 조회를 위한 mock 대응
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

func TestPriceCollectsDailyRange(t *testing.T) {
	tests := []struct {
		name             string
		rows             []map[string]any
		wantRangePoints  float64
		wantRangePercent float64
		wantYearHigh     bool
	}{
		{name: "daily row", rows: []map[string]any{{
			"stck_bsop_date": "20260515", "bstp_nmix_prpr": "7950",
			"bstp_nmix_oprc": "7900", "bstp_nmix_hgpr": "8050", "bstp_nmix_lwpr": "7600",
			"stck_prdy_clpr": "8000", "dryy_bstp_nmix_hgpr": "8050",
		}}, wantRangePoints: 450, wantRangePercent: 5.625, wantYearHigh: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectPrice(context.Background(), fakeStock{dailyRows: tt.rows}, "20260515")
			if err != nil {
				t.Fatal(err)
			}
			if got.RangePoints != tt.wantRangePoints || got.RangePercent != tt.wantRangePercent || got.YearHigh != tt.wantYearHigh {
				t.Fatalf("price section = %+v", got)
			}
		})
	}
}

func TestFlowConvertsTradeAmountsToEok(t *testing.T) {
	tests := []struct {
		name                                     string
		row                                      map[string]any
		wantForeign, wantInstitution, wantRetail float64
	}{
		{name: "million KRW to eok", row: map[string]any{
			"frgn_ntby_tr_pbmn": "-4834200", "orgn_ntby_tr_pbmn": "-734000", "prsn_ntby_tr_pbmn": "5419800",
		}, wantForeign: -48342, wantInstitution: -7340, wantRetail: 54198},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &auth.RESTResponse{Body: map[string]any{"output": []any{tt.row}}}
			got, err := collectFlow(context.Background(), fakeStock{investor: resp}, "20260515")
			if err != nil {
				t.Fatal(err)
			}
			if got.ForeignEok != tt.wantForeign || got.InstitutionEok != tt.wantInstitution || got.IndividualEok != tt.wantRetail {
				t.Fatalf("flow section = %+v", got)
			}
		})
	}
}

func TestImpactComputesRatiosAndSidecar(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "ratios and manual sidecar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			semiSell := -42000.0
			stock := fakeStock{cap: &domesticstock.KOSPIMarketCapSummary{TotalMarketCap: 7_100_000}}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output1": map[string]any{"futs_prdy_ctrt": "-5.09"}}}}
			got := collectImpact(context.Background(), Deps{DomesticStock: stock, DomesticFuture: futures}, "20260515", &FlowSection{ForeignEok: -48342}, &PriceSection{TradingValueEok: 200_000}, Options{
				SidecarStatus: "triggered", SidecarTime: "13:28:49", SemiconductorForeignNetSellEok: &semiSell,
			})
			if got.ForeignNetFlowToMarketCap == nil || *got.ForeignNetFlowToMarketCap > -0.67 || *got.ForeignNetFlowToMarketCap < -0.69 {
				t.Fatalf("foreign sell ratio = %+v", got.ForeignNetFlowToMarketCap)
			}

			if got.SemiconductorSellConcentrationPct == nil || *got.SemiconductorSellConcentrationPct < 86 {
				t.Fatalf("semiconductor ratio = %+v", got.SemiconductorSellConcentrationPct)
			}
			if got.FuturesChangePercent == nil || *got.FuturesChangePercent != -5.09 || got.SidecarStatus != "triggered" {
				t.Fatalf("impact section = %+v", got)
			}
		})
	}
}

func TestGlobalKeepsPartialQuotes(t *testing.T) {
	tests := []struct {
		name       string
		quotes     map[string]yahoo.Quote
		err        error
		wantReason string
	}{
		{name: "partial quotes", quotes: map[string]yahoo.Quote{
			"KRW=X": {Symbol: "KRW=X", Price: 1494.2, ChangePercent: 0.51},
		}, err: errors.New("yahoo quote missing: BTC-USD"), wantReason: "BTC-USD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectGlobal(context.Background(), fakeYahoo{quotes: tt.quotes, err: tt.err})
			if err != nil || !strings.Contains(got.Reason, tt.wantReason) {
				t.Fatalf("global = %+v err=%v", got, err)
			}
		})
	}
}

func TestVolatilityPrefersKISAndUsesOppositeDirectionForDecoupling(t *testing.T) {
	stock := fakeStock{
		vkospi: &auth.RESTResponse{Body: map[string]any{"output": map[string]any{
			"bstp_nmix_prpr": "28.50", "bstp_nmix_prdy_ctrt": "12.30",
		}}},
		vkospiDaily: []map[string]any{
			{"bstp_nmix_prpr": "28.50"}, {"bstp_nmix_prpr": "25.00"},
			{"bstp_nmix_prpr": "24.00"}, {"bstp_nmix_prpr": "23.00"}, {"bstp_nmix_prpr": "22.00"},
		},
	}
	naverClient := fakeNaver{quote: &naver.IndexQuote{Price: 99, ChangePercent: -20}}
	got := collectVolatility(context.Background(), stock, naverClient, fakeYahoo{}, -3, "20260710", Options{})
	if got.VKOSPI != 28.5 || got.Source != "KIS" {
		t.Fatalf("expected KIS VKOSPI 28.5, got %+v", got)
	}
	if !got.DecouplingFlag {
		t.Fatalf("expected falling index and rising VKOSPI to be decoupling")
	}
	if isDecoupling(-3, -12.3) {
		t.Fatalf("same-direction index/VKOSPI moves must not be decoupling")
	}
}

func TestMacroComputesKRWMonthStartAndTNXRender(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "month start and tnx scale"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := 1430.0
			s := &Snapshot{Macro: &MacroSection{USDKRWMonthStart: &start, USDKRWMonthStartPct: ptr(4.49), Quotes: map[string]yahoo.Quote{
				"KRW=X": {Price: 1494.2}, "CL=F": {Price: 102.34}, "^TNX": {Price: 4.52}, // raw = percent
			}}}
			out := Render(s)
			if !strings.Contains(out, "USD/KRW: 1,494.20") || !strings.Contains(out, "미국 10년물: 4.52%") {
				t.Fatalf("rendered macro missing values:\n%s", out)
			}
		})
	}
}

func TestCumulativeManualAndMissingValues(t *testing.T) {
	tests := []struct {
		name    string
		monthly float64
	}{
		{name: "manual monthly only", monthly: -202000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectCumulative(context.Background(), nil, "20260515", Options{MonthlyForeignNetSellEok: &tt.monthly})
			if got.MonthlyForeignNetSellEok == nil || got.ForeignHoldingReason == "" || got.CapRatioReason == "" {
				t.Fatalf("cumulative section = %+v", got)
			}
		})
	}
}

func TestLateSession(t *testing.T) {
	t.Run("capitulation event detected", func(t *testing.T) {
		// Mock KIS REST responses
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

		stock := fakeStock{
			program:  progResp,
			compProg: compProgResp,
			timeFlow: timeFlowResp,
		}

		futures := fakeFuture{
			resp: &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"futs_prpr": "349.5"},
					},
				},
			},
		}

		deps := Deps{
			DomesticStock:  stock,
			DomesticFuture: futures,
		}

		priceSec := &PriceSection{
			High:  100,
			Low:   90,
			Close: 90.1,
		}

		got, err := collectLateSession(context.Background(), deps, "20260515", priceSec, Options{})
		if err != nil {
			t.Fatal(err)
		}

		if got.SpotPrice != 350.0 {
			t.Errorf("expected SpotPrice 350.0, got %.2f", got.SpotPrice)
		}
		if got.FuturesPrice != 349.5 {
			t.Errorf("expected FuturesPrice 349.5, got %.2f", got.FuturesPrice)
		}
		if got.BasisPoint != -0.5 {
			t.Errorf("expected BasisPoint -0.5, got %.2f", got.BasisPoint)
		}
		if got.KOSPINetNonArbitrageForeign != -400.0 {
			t.Errorf("expected non-arbitrage foreign -400.0, got %.2f", got.KOSPINetNonArbitrageForeign)
		}
		if got.LateProgramNetEok == nil || *got.LateProgramNetEok != -800.0 {
			t.Errorf("expected late program flow -800.0, got %v", got.LateProgramNetEok)
		}
		if got.CloseSessionProgramNetEok == nil || *got.CloseSessionProgramNetEok != -400.0 {
			t.Errorf("expected close session program -400.0, got %v", got.CloseSessionProgramNetEok)
		}
		if got.CloseSessionForeignNetEok == nil || *got.CloseSessionForeignNetEok != -300.0 {
			t.Errorf("expected close session foreign -300.0, got %v", got.CloseSessionForeignNetEok)
		}
		if !got.PatternDetected {
			t.Errorf("expected PatternDetected to be true, got false")
		}
		if got.PrimaryPattern != "Late-Session Capitulation" {
			t.Errorf("expected PrimaryPattern to be 'Late-Session Capitulation', got '%s'", got.PrimaryPattern)
		}
	})

	t.Run("short squeeze event detected", func(t *testing.T) {
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
		if err != nil {
			t.Fatal(err)
		}
		if !got.PatternDetected {
			t.Errorf("expected PatternDetected to be true")
		}
		if got.PrimaryPattern != "Late-Session Short Squeeze" {
			t.Errorf("expected PrimaryPattern to be 'Late-Session Short Squeeze', got '%s'", got.PrimaryPattern)
		}
	})

	t.Run("window dressing event detected", func(t *testing.T) {
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

		// 20260630: Quarter End (June 30)
		got, err := collectLateSession(context.Background(), deps, "20260630", priceSec, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !got.PatternDetected {
			t.Errorf("expected PatternDetected to be true")
		}
		if got.PrimaryPattern != "Window Dressing" {
			t.Errorf("expected PrimaryPattern to be 'Window Dressing', got '%s'", got.PrimaryPattern)
		}
	})

	t.Run("etf rebalancing impact event detected", func(t *testing.T) {
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

		// 20260528: Rebalancing Day (>= 25 of May)
		got, err := collectLateSession(context.Background(), deps, "20260528", priceSec, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !got.PatternDetected {
			t.Errorf("expected PatternDetected to be true")
		}
		if got.PrimaryPattern != "ETF Rebalancing Impact" {
			t.Errorf("expected PrimaryPattern to be 'ETF Rebalancing Impact', got '%s'", got.PrimaryPattern)
		}
	})

	t.Run("expiration basis arbitrage event detected", func(t *testing.T) {
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

		// 20260611: Expiration Day (second Thursday of June)
		got, err := collectLateSession(context.Background(), deps, "20260611", priceSec, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !got.PatternDetected {
			t.Errorf("expected PatternDetected to be true")
		}
		if got.PrimaryPattern != "Expiration Basis Arbitrage" {
			t.Errorf("expected PrimaryPattern to be 'Expiration Basis Arbitrage', got '%s'", got.PrimaryPattern)
		}
	})
}

// === Bug Fix Tests ===

func TestRenderPriceUsesAPIPreviousClose(t *testing.T) {
	t.Run("uses pr.PreviousClose as authoritative value", func(t *testing.T) {
		s := &Snapshot{
			Price: &PriceSection{
				Date: "20260619", Open: 9100, High: 9385, Low: 9010,
				Close: 9040.36, PreviousClose: 9063.84, // KIS API의 전일 종가 (올바른 6/18 종가)
			},
		}
		out := Render(s)
		// KIS API의 전일 종가 9,063.84가 표시되어야 함
		if !strings.Contains(out, "9,063.84") {
			t.Fatalf("expected authoritative prev close 9,063.84 in output, got:\n%s", out)
		}
		// 변화폭이 음수 (-23.48)로 표시되어야 함
		if !strings.Contains(out, "-23.48") {
			t.Fatalf("expected negative change -23.48 in output, got:\n%s", out)
		}
	})

	t.Run("warns when saved JSON diverges from API prev close", func(t *testing.T) {
		s := &Snapshot{
			Price: &PriceSection{
				Date: "20260619", Open: 9100, High: 9385, Low: 9010,
				Close: 9040.36, PreviousClose: 9063.84,
			},
		}
		prev := &SnapshotJSON{
			Date: "20260618",
			Price: &PriceSection{
				Close: 8864.24, // 잘못된 값 (실제로는 6/17 종가)
			},
		}
		out := Render(s, prev)
		if !strings.Contains(out, "⚠") || !strings.Contains(out, "불일치") {
			t.Fatalf("expected divergence warning in output, got:\n%s", out)
		}
		// 여전히 올바른 전일 종가를 표시해야 함
		if !strings.Contains(out, "9,063.84") {
			t.Fatalf("should still show authoritative prev close, got:\n%s", out)
		}
	})

	t.Run("no warning when saved JSON matches API prev close", func(t *testing.T) {
		s := &Snapshot{
			Price: &PriceSection{
				Date: "20260619", Open: 9100, High: 9385, Low: 9010,
				Close: 9040.36, PreviousClose: 9063.84,
			},
		}
		prev := &SnapshotJSON{
			Date: "20260618",
			Price: &PriceSection{
				Close: 9063.84, // 올바른 값
			},
		}
		out := Render(s, prev)
		if strings.Contains(out, "불일치") {
			t.Fatalf("should not warn when values match, got:\n%s", out)
		}
	})
}

func TestImpactBasisRemovedFromSection3(t *testing.T) {
	t.Run("impact section no longer has basis fields", func(t *testing.T) {
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
		// Section 3에 베이시스가 LateSession 참조로 표시되어야 함
		if !strings.Contains(out, "Section 11 참조") {
			t.Fatalf("expected Section 11 reference in impact basis, got:\n%s", out)
		}
		// Section 11의 올바른 베이시스 값이 표시되어야 함
		if !strings.Contains(out, "14.1") {
			t.Fatalf("expected correct basis from Section 11, got:\n%s", out)
		}
	})
}

func TestProgramTradeTotalFallback(t *testing.T) {
	t.Run("computes total when 합계 row is missing", func(t *testing.T) {
		// API 응답에 "합계" 행이 없는 경우
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
		if err := fillProgramTradeToday(context.Background(), Deps{DomesticStock: stock}, sec); err != nil {
			t.Fatal(err)
		}
		// 외국인 = 414900/100 = 4149
		if sec.KOSPINetNonArbitrageForeign != 4149.0 {
			t.Errorf("expected foreign 4149, got %.2f", sec.KOSPINetNonArbitrageForeign)
		}
		// 기관 = -880600/100 = -8806
		if sec.KOSPINetNonArbitrageOrgan != -8806.0 {
			t.Errorf("expected organ -8806, got %.2f", sec.KOSPINetNonArbitrageOrgan)
		}
		// Total은 fallback 합산 = 4149 + (-8806) + 5631 = 974
		expectedTotal := 4149.0 + (-8806.0) + 5631.0
		if sec.KOSPINetNonArbitrageTotal != expectedTotal {
			t.Errorf("expected fallback total %.0f, got %.2f", expectedTotal, sec.KOSPINetNonArbitrageTotal)
		}
	})

	t.Run("uses 합계 row when present", func(t *testing.T) {
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
		if err := fillProgramTradeToday(context.Background(), Deps{DomesticStock: stock}, sec); err != nil {
			t.Fatal(err)
		}
		// Total은 API의 합계 행 값 사용 = -100000/100 = -1000
		if sec.KOSPINetNonArbitrageTotal != -1000.0 {
			t.Errorf("expected total -1000 from 합계 row, got %.2f", sec.KOSPINetNonArbitrageTotal)
		}
	})

	t.Run("matches 계 row name without 합", func(t *testing.T) {
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
		if err := fillProgramTradeToday(context.Background(), Deps{DomesticStock: stock}, sec); err != nil {
			t.Fatal(err)
		}
		if sec.KOSPINetNonArbitrageTotal != 2000.0 {
			t.Errorf("expected total 2000 from '계' row, got %.2f", sec.KOSPINetNonArbitrageTotal)
		}
	})
}

func TestEBAGatesAndStaleDatesAndMeltdownRegime(t *testing.T) {
	t.Run("EBA calendar gate prevents trigger on non-expiration days", func(t *testing.T) {
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
		if err != nil {
			t.Fatal(err)
		}
		if got.PrimaryPattern == "Expiration Basis Arbitrage" {
			t.Errorf("EBA should not trigger on a non-expiration day")
		}
	})

	t.Run("EBA volume gate prevents trigger when program trades are small", func(t *testing.T) {
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
		if err != nil {
			t.Fatal(err)
		}
		if got.PrimaryPattern == "Expiration Basis Arbitrage" {
			t.Errorf("EBA should not trigger if program trade volume is small")
		}
	})

	t.Run("Regime and Risk index update on market crash", func(t *testing.T) {
		price := &PriceSection{
			Close:         7648.09,
			PreviousClose: 8303.41, // -7.89%
			High:          8136.28,
			Low:           7616.33,
		}
		impact := &ImpactSection{
			SidecarStatus: "triggered",
		}

		phase := classifyPhase(price, impact, nil, nil)
		if !strings.Contains(phase, "패닉") {
			t.Errorf("Expected panic phase for market crash, got %q", phase)
		}

		reg := collectRegime(context.Background(), fakeYahoo{}, price, nil, impact, nil, nil)
		if reg.DomesticMarketStressIdx < 8.0 {
			t.Errorf("Expected domestic market stress floor to be >= 8.0 on crash day, got %.1f", reg.DomesticMarketStressIdx)
		}
	})

	t.Run("Stale date comparison displays 미갱신", func(t *testing.T) {
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
		if !strings.Contains(out, "미갱신") {
			t.Errorf("Expected '미갱신' in output for stale dates, got:\n%s", out)
		}
	})
}
