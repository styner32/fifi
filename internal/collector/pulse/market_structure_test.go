package pulse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/domesticstock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type programFake struct{ resp *auth.RESTResponse }

func (f programFake) CompProgramTradeToday(context.Context, string) (*auth.RESTResponse, error) {
	return f.resp, nil
}

type futureFake struct{ resp *auth.RESTResponse }

func (f futureFake) ResolveNearMonthKOSPI200Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error) {
	return &domesticfutureoption.ResolvedContract{Record: domesticfutureoption.MasterRecord{ShortCode: "A01609", Name: "F 202609"}}, nil
}
func (f futureFake) ResolveNearMonthKOSDAQ150Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error) {
	return &domesticfutureoption.ResolvedContract{Record: domesticfutureoption.MasterRecord{ShortCode: "A06609", Name: "코스닥150F 202609"}}, nil
}
func (f futureFake) InquirePrice(context.Context, string, string) (*auth.RESTResponse, error) {
	return f.resp, nil
}

type contributionFake struct {
	summary *domesticstock.KOSPIMarketCapSummary
	changes map[string]float64
}

func (f contributionFake) KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error) {
	return f.summary, nil
}
func (f contributionFake) InquirePrice(_ context.Context, code string) (*auth.RESTResponse, error) {
	change, ok := f.changes[code]
	if !ok {
		return nil, fmt.Errorf("missing %s", code)
	}
	return &auth.RESTResponse{Body: map[string]any{"output": map[string]any{"prdy_ctrt": fts(change)}}}, nil
}

type vkospiFake struct{ resp *auth.RESTResponse }

func (f vkospiFake) ResolveVKOSPICode(context.Context, []string) (string, error) { return "0503", nil }
func (f vkospiFake) InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error) {
	return f.resp, nil
}

var _ = Describe("market structure collectors", func() {
	It("프로그램매매 최신 행을 선택하고 백만원을 억원으로 변환", func() {
		resp := &auth.RESTResponse{Body: map[string]any{"output": []any{
			map[string]any{"bsop_hour": "130000", "arbt_smtn_ntby_tr_pbmn": "-1200", "nabt_smtn_ntby_tr_pbmn": "3500", "whol_smtn_ntby_tr_pbmn": "2300"},
			map[string]any{"bsop_hour": "131000", "arbt_smtn_ntby_tr_pbmn": "-2500", "nabt_smtn_ntby_tr_pbmn": "5000", "whol_smtn_ntby_tr_pbmn": "2500"},
		}}}
		got, err := collectProgramTrade(context.Background(), programFake{resp}, "K")
		Expect(err).NotTo(HaveOccurred())
		Expect(got.OK).To(BeTrue())
		Expect(got.AsOf).To(Equal("131000"))
		Expect(got.Arbitrage).To(Equal(-25.0))
		Expect(got.NonArbitrage).To(Equal(50.0))
		Expect(got.Total).To(Equal(25.0))
	})

	It("장 마감 후 반복 프로그램 행은 장마감으로 표시", func() {
		resp := &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{
			"bsop_hour": "172100", "arbt_smtn_ntby_tr_pbmn": "0", "nabt_smtn_ntby_tr_pbmn": "100", "whol_smtn_ntby_tr_pbmn": "100",
		}}}}
		got, err := collectProgramTrade(context.Background(), programFake{resp}, "K")
		Expect(err).NotTo(HaveOccurred())
		Expect(got.AsOf).To(Equal("close"))
	})

	It("KOSPI200 현물과 선물로 시장 베이시스를 교차 검증", func() {
		resp := &auth.RESTResponse{Body: map[string]any{
			"output1": map[string]any{"futs_prpr": "1300.15", "futs_prdy_clpr": "1381.40", "futs_prdy_ctrt": "-5.88", "mrkt_basis": "1.13"},
			"output3": map[string]any{"bstp_nmix_prpr": "1299.02", "bstp_nmix_prdy_ctrt": "-5.80"},
		}}
		got, err := collectIndexFuture(context.Background(), futureFake{resp}, "20260702", "KOSPI200", time.Now())
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Basis).To(BeNumerically("~", 1.13, 0.001))
		Expect(got.BasisMatch).To(BeTrue())
	})

	It("CB는 하락 방향만 계산하고 저점 기준 근접도를 보존", func() {
		idx := IndexLevel{PrevClose: 8303.45, Price: 7844.28, Low: 7723.57, ChangePct: -5.53, OK: true}
		safety := buildMarketSafety(time.Now(), "20260702", idx, IndexLevel{}, IndexFutureSnapshot{}, IndexFutureSnapshot{}, nil)
		Expect(safety.CircuitBreakers).To(HaveLen(1))
		phase1 := safety.CircuitBreakers[0].Levels[0]
		Expect(phase1.CurrentGapPct).To(BeNumerically("~", 2.47, 0.01))
		Expect(phase1.LowGapPct).To(BeNumerically("~", 1.02, 0.02))
		Expect(phase1.LowReached).To(BeFalse())

		// 상승 시 시나리오 (+5.17% 일 때 -8% 임계까지의 거리는 13.17%p)
		idxPositive := IndexLevel{PrevClose: 1000.0, Price: 1051.7, Low: 1020.0, ChangePct: 5.17, OK: true}
		safetyPos := buildMarketSafety(time.Now(), "20260702", idxPositive, IndexLevel{}, IndexFutureSnapshot{}, IndexFutureSnapshot{}, nil)
		Expect(safetyPos.CircuitBreakers).To(HaveLen(1))
		phase1Pos := safetyPos.CircuitBreakers[0].Levels[0]
		Expect(phase1Pos.CurrentGapPct).To(BeNumerically("~", 13.17, 0.01))
		Expect(phase1Pos.LowGapPct).To(BeNumerically("~", 10.00, 0.01))
	})

	It("KOSDAQ 사이드카는 선물과 현물 조건을 같은 방향으로 모두 요구", func() {
		f := IndexFutureSnapshot{Code: "A06609", ChangePct: -6.2, SpotChangePct: -3.1, OK: true}
		got := buildSidecar("KOSDAQ", f, 6, 3, nil)
		Expect(got.ThresholdReached).To(BeTrue())
		f.SpotChangePct = 3.1
		Expect(buildSidecar("KOSDAQ", f, 6, 3, nil).ThresholdReached).To(BeFalse())
	})

	It("선물 급등 시 SIDECAR_SELL과 SIDECAR_BUY 임계 간격을 정확히 산출", func() {
		now := time.Date(2026, 7, 21, 13, 13, 54, 0, time.Local)
		k200 := IndexFutureSnapshot{Code: "A01609", ChangePct: 5.46, OK: true}
		safety := buildMarketSafety(now, "20260721", IndexLevel{}, IndexLevel{}, k200, IndexFutureSnapshot{}, nil)

		var sellDev, buyDev *SafetyDeviceStatus
		for i := range safety.Devices {
			if safety.Devices[i].Market == "KOSPI" && safety.Devices[i].Device == "SIDECAR_SELL" {
				sellDev = &safety.Devices[i]
			}
			if safety.Devices[i].Market == "KOSPI" && safety.Devices[i].Device == "SIDECAR_BUY" {
				buyDev = &safety.Devices[i]
			}
		}

		Expect(sellDev).NotTo(BeNil())
		Expect(sellDev.State).To(Equal("ELIGIBLE"))
		Expect(sellDev.ThresholdDistancePct).NotTo(BeNil())
		Expect(*sellDev.ThresholdDistancePct).To(BeNumerically("~", 10.46, 0.01))

		Expect(buyDev).NotTo(BeNil())
		Expect(buyDev.State).To(Equal("CONDITION_OBSERVED"))
	})

	It("시총 상위 종목의 추정 포인트 기여도를 계산", func() {
		fake := contributionFake{
			summary: &domesticstock.KOSPIMarketCapSummary{TotalMarketCap: 1000, Constituents: []domesticstock.KOSPIMarketCapConstituent{
				{Code: "005930", Name: "삼성전자", MarketCap: 400},
				{Code: "000660", Name: "SK하이닉스", MarketCap: 200},
			}},
			changes: map[string]float64{"005930": -5, "000660": -10},
		}
		got, err := collectContributions(context.Background(), fake, "20260702", IndexLevel{PrevClose: 1000, OK: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(2))
		Expect(got[0].PointImpact).To(BeNumerically("~", -20, 0.001))
		Expect(got[1].PointImpact).To(BeNumerically("~", -20, 0.001))
	})

	It("VKOSPI는 KIS 응답을 우선 사용", func() {
		resp := &auth.RESTResponse{Body: map[string]any{
			"rt_cd": "0",
			"output": map[string]any{
				"bstp_nmix_prpr": "28.50", "bstp_nmix_prdy_ctrt": "12.30",
			},
		}}
		got, err := collectVKOSPI(context.Background(), vkospiFake{resp}, nil, time.Now())
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Source).To(Equal("KIS"))
		Expect(got.Value).To(Equal(28.5))
	})
})

var _ = Describe("flow rates", func() {
	It("불균등 구간을 시간당 속도로 비교하고 부호 반전을 표시", func() {
		current := &FlowDelta{Elapsed: 90, Foreign: -900}
		twoHour := &FlowDelta{Elapsed: 150, Foreign: -300}
		Expect(FlowAcceleration(current, twoHour, func(d *FlowDelta) float64 { return d.Foreign })).To(Equal("매도전환"))
	})

	It("분석 문구가 음수 수급을 순매도로 표시", func() {
		p := &Pulse{KOSPI: Market{
			Flow:        FlowSnapshot{OK: true, Foreign: -5000},
			FlowDelta1h: &FlowDelta{Elapsed: 98, Foreign: -4838},
		}}
		combined := strings.Join(analyzeFlowLeader(p), "\n")
		Expect(combined).To(ContainSubstring("최근 98m"))
		Expect(combined).To(ContainSubstring("순매도"))
		Expect(combined).NotTo(ContainSubstring("순매수"))
	})

	It("렌더가 거래대금 화살표를 제거하고 no-save 경로와 미국채 bp를 표시", func() {
		move1h := 0.25
		usdkrwMove := -0.17
		p := &Pulse{
			Now: time.Date(2026, 7, 2, 13, 40, 0, 0, kstLocation), Date: "20260702",
			StoreDir: "/tmp/custom-pulse",
			KOSPI:    Market{Name: "KOSPI", Index: IndexLevel{Price: 100, PrevClose: 105, ChangePct: -4.76, TradingValue: 12000, OK: true}},
			KOSDAQ:   Market{Name: "KOSDAQ"},
			USDKRW:   Window{Symbol: "KRW=X", Label: "원/달러", Current: 1548.13, ChangePct: -0.17, Move1hPct: &usdkrwMove, OK: true},
			Macro:    []Window{{Symbol: "^TNX", Label: "미국채10Y", Current: 4.5, ChangePct: 1, Move1hPct: &move1h, OK: true}},
			Errors:   map[string]string{},
		}
		out := Render(p)
		Expect(out).To(ContainSubstring("거래대금 1.20조"))
		Expect(out).NotTo(ContainSubstring("거래대금 ▲"))
		Expect(out).To(ContainSubstring("원/달러"))
		Expect(out).To(ContainSubstring("전일 ▼-0.17%"))
		Expect(out).To(ContainSubstring("미국채10Y"))
		Expect(out).To(ContainSubstring("bp"))
		Expect(out).To(ContainSubstring("저장 안 함 · 대상 경로 /tmp/custom-pulse/pulse_20260702.jsonl"))
	})

	It("수급 렌더에 기타법인 및 기관 7개 세부항목(금융투자, 보험, 투신, 기타금융, 은행, 연기금, 사모)과 합계가 정상 표시", func() {
		p := &Pulse{
			Now:  time.Date(2026, 8, 31, 12, 4, 0, 0, kstLocation),
			Date: "20260831",
			KOSPI: Market{
				Name: "KOSPI",
				Flow: FlowSnapshot{
					Foreign:     -6219,
					Institution: -6427,
					Individual:  4682,
					EtcCorp:     7964,
					FinInvest:   -2683,
					Insurance:   20,
					InvTrust:    -347,
					EtcFin:      81,
					Bank:        43,
					Pension:     147,
					PrivEquity:  -3688,
					OK:          true,
				},
			},
			KOSDAQ: Market{
				Name: "KOSDAQ",
				Flow: FlowSnapshot{
					Foreign:     -1349,
					Institution: -1953,
					Individual:  3327,
					EtcCorp:     -25,
					FinInvest:   -1000,
					Insurance:   -53,
					InvTrust:    -200,
					EtcFin:      0,
					Bank:        0,
					Pension:     -200,
					PrivEquity:  -500,
					OK:          true,
				},
			},
			Errors: map[string]string{},
		}
		out := Render(p)
		// KOSPI 누적 라인에 기타법인 및 합계 0억 포함 검증
		Expect(out).To(ContainSubstring("KOSPI    누적: 외국인 ▼-6219억 · 기관 ▼-6427억 · 개인 ▲+4682억 · 기타법인 ▲+7964억 (합계 0억)"))
		// KOSPI 기관 세부 7개 항목 및 세부 합계 포함 검증
		Expect(out).To(ContainSubstring("└ 기관 세부(누적): 금융투자 ▼-2683억 · 보험 ▲+20억 · 투신 ▼-347억 · 기타금융 ▲+81억 · 은행 ▲+43억 · 연기금 ▲+147억 · 사모 ▼-3688억 (합계 ▼-6427억)"))
		// KOSDAQ 누적 라인에 기타법인 및 합계 0억 포함 검증
		Expect(out).To(ContainSubstring("KOSDAQ   누적: 외국인 ▼-1349억 · 기관 ▼-1953억 · 개인 ▲+3327억 · 기타법인 ▼-25억 (합계 0억)"))
	})

	It("수급 합계 불일치 시 경고 표시 출력", func() {
		p := &Pulse{
			Now:  time.Date(2026, 8, 31, 12, 4, 0, 0, kstLocation),
			Date: "20260831",
			KOSPI: Market{
				Name: "KOSPI",
				Flow: FlowSnapshot{
					Foreign:     -6219,
					Institution: -6427,
					Individual:  4682,
					EtcCorp:     0, // 기타법인 누락 시 잔차 발생
					FinInvest:   -2683,
					OK:          true,
				},
			},
			KOSDAQ: Market{Name: "KOSDAQ"},
			Errors: map[string]string{},
		}
		out := Render(p)
		Expect(out).To(ContainSubstring("⚠️ 합계 불일치"))
	})

	It("시장폭, 단순스프레드(raw_spread), 환율 기준가, 사이드카 세부 등락률이 정상 렌더링", func() {
		p := &Pulse{
			Now:  time.Date(2026, 8, 31, 14, 40, 0, 0, kstLocation),
			Date: "20260831",
			KOSPI: Market{
				Name: "KOSPI",
				Index: IndexLevel{
					Price:      6759.44,
					PrevClose:  6788.0,
					ChangePct:  -0.43,
					Open:       6613.58,
					High:       6808.07,
					Low:        6547.76,
					UpperLimit: 1,
					Advancers:  312,
					Unchanged:  50,
					Decliners:  565,
					LowerLimit: 0,
					TotalCount: 928,
					OK:         true,
				},
			},
			KOSDAQ: Market{Name: "KOSDAQ"},
			KOSPI200Future: IndexFutureSnapshot{
				Code:          "A01609",
				Price:         1062.70,
				SpotPrice:     1061.34,
				Basis:         1.36,
				RawSpread:     1.36,
				ChangePct:     -0.50,
				SpotChangePct: -0.40,
				OK:            true,
			},
			USDKRW: Window{
				Symbol:    "KRW=X",
				Label:     "원/달러(Yahoo 역외스팟)",
				Current:   1372.58,
				PrevClose: 1375.60,
				ChangePct: -0.22,
				OK:        true,
			},
			Safety: MarketSafety{
				Devices: []SafetyDeviceStatus{
					{
						Market:               "KOSPI",
						Device:               "SIDECAR_SELL",
						Threshold:            5.0,
						FuturesChangePct:     ptr(-0.50),
						ThresholdDistancePct: ptr(4.50),
						State:                "ELIGIBLE",
						EligibilityReason:    "발동 가능 시간대",
					},
					{
						Market:            "KOSDAQ",
						Device:            "SIDECAR_SELL",
						Threshold:         6.0,
						SpotThreshold:     ptr(3.0),
						FuturesChangePct:  ptr(-1.97),
						SpotChangePct:     ptr(-1.16),
						FuturesGapPct:     ptr(4.03),
						SpotGapPct:        ptr(1.84),
						State:             "ELIGIBLE",
						EligibilityReason: "발동 가능 시간대",
					},
				},
			},
			Errors: map[string]string{},
		}

		out := Render(p)
		// 1. 시장 폭 전체 종목 수 및 상승비율 검증
		Expect(out).To(ContainSubstring("상승 312 (상한 1) · 보합 50 · 하락 565 · 총 928 (상승비율 33.7%)"))
		// 2. 단순스프레드(raw_spread) 명칭 검증
		Expect(out).To(ContainSubstring("단순스프레드(raw_spread) +1.36p (콘탱고)"))
		// 3. 환율 기준가 검증
		Expect(out).To(ContainSubstring("(기준 1375.60원)"))
		// 4. 사이드카 선물/현물 세부 등락률 및 간격 검증
		Expect(out).To(ContainSubstring("[KOSPI] SIDECAR_SELL (선물 -0.50% [임계 -5.0%]): 상태 ELIGIBLE (간격 4.50%p)"))
		Expect(out).To(ContainSubstring("[KOSDAQ] SIDECAR_SELL (선물 -1.97% [임계 -6.0%] · 현물 -1.16% [임계 -3.0%]): 상태 ELIGIBLE (간격 선물 4.03%p / 현물 1.84%p)"))
	})
})
