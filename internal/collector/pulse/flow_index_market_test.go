package pulse

import (
	"context"
	"fmt"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/external/yahoo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ──────────────────────────────────────────────────────────────────────────────
// Fake implementations (internal package, white-box)
// ──────────────────────────────────────────────────────────────────────────────

type testStock struct {
	flowResp  map[string]*auth.RESTResponse
	indexResp map[string]*auth.RESTResponse
}

func (f testStock) InquireInvestorTimeByMarket(_ context.Context, marketDiv, _ string) (*auth.RESTResponse, error) {
	if resp, ok := f.flowResp[marketDiv]; ok {
		return resp, nil
	}
	return nil, nil
}

func (f testStock) InquireIndexPrice(_ context.Context, indexCode string) (*auth.RESTResponse, error) {
	if resp, ok := f.indexResp[indexCode]; ok {
		return resp, nil
	}
	return nil, nil
}

func flowResp(foreign, institution, individual float64) *auth.RESTResponse {
	return &auth.RESTResponse{Body: map[string]any{
		"output": []any{map[string]any{
			"frgn_ntby_tr_pbmn":     fts(foreign),
			"orgn_ntby_tr_pbmn":     fts(institution),
			"prsn_ntby_tr_pbmn":     fts(individual),
			"scrt_ntby_tr_pbmn":     "0",
			"ivtr_ntby_tr_pbmn":     "0",
			"fund_ntby_tr_pbmn":     "0",
			"pe_fund_ntby_tr_pbmn":  "0",
			"insu_ntby_tr_pbmn":     "0",
			"bank_ntby_tr_pbmn":     "0",
			"etc_corp_ntby_tr_pbmn": "0",
		}},
	}}
}

func idxResp(price, prdyVrss, open, high, low, acml float64, up, dn int) *auth.RESTResponse {
	ctrt := 0.0
	if price-prdyVrss != 0 {
		ctrt = prdyVrss / (price - prdyVrss) * 100
	}
	return &auth.RESTResponse{Body: map[string]any{
		"output": []any{map[string]any{
			"bstp_nmix_prpr":      fts(price),
			"bstp_nmix_prdy_vrss": fts(prdyVrss),
			"bstp_nmix_prdy_ctrt": fts(ctrt),
			"bstp_nmix_oprc":      fts(open),
			"bstp_nmix_hgpr":      fts(high),
			"bstp_nmix_lwpr":      fts(low),
			"acml_tr_pbmn":        fts(acml),
			"ascn_issu_cnt":       fts(float64(up)),
			"down_issu_cnt":       fts(float64(dn)),
			"stnr_issu_cnt":       "5",
		}},
	}}
}

func fts(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.2f", f)
}

// ──────────────────────────────────────────────────────────────────────────────
// flow_test
// ──────────────────────────────────────────────────────────────────────────────

var _ = Describe("collectFlow", func() {
	It("백만원→억원 변환 및 부호 검증", func() {
		stock := testStock{
			flowResp: map[string]*auth.RESTResponse{
				"KSP": flowResp(-3_556_053, -1_540_481, 5_103_306),
			},
		}
		snap, err := collectFlow(context.Background(), stock, "KSP", "0001", time.Now())
		Expect(err).To(BeNil())
		Expect(snap.OK).To(BeTrue())
		Expect(snap.Foreign).To(BeNumerically("~", -35560.53, 0.01))
		Expect(snap.Institution).To(BeNumerically("~", -15404.81, 0.01))
		Expect(snap.Individual).To(BeNumerically("~", 51033.06, 0.01))
	})

	It("output 행 없으면 에러", func() {
		stock := testStock{
			flowResp: map[string]*auth.RESTResponse{
				"KSP": {Body: map[string]any{}},
			},
		}
		_, err := collectFlow(context.Background(), stock, "KSP", "0001", time.Now())
		Expect(err).NotTo(BeNil())
	})

	It("KOSDAQ 수급도 올바르게 파싱", func() {
		stock := testStock{
			flowResp: map[string]*auth.RESTResponse{
				"KSQ": flowResp(-1800, 114200, -114000),
			},
		}
		snap, err := collectFlow(context.Background(), stock, "KSQ", "1001", time.Now())
		Expect(err).To(BeNil())
		Expect(snap.Foreign).To(BeNumerically("~", -18.0, 0.01))
		Expect(snap.Institution).To(BeNumerically("~", 1142.0, 0.01))
	})
})

// ──────────────────────────────────────────────────────────────────────────────
// index_test
// ──────────────────────────────────────────────────────────────────────────────

var _ = Describe("collectIndex", func() {
	It("전일종가=현재가-전일대비, 거래대금 억 변환", func() {
		stock := testStock{
			indexResp: map[string]*auth.RESTResponse{
				"0001": idxResp(8565.06, -549.49, 9083.54, 9175.45, 8511.14, 36_585_627, 87, 823),
			},
		}
		idx, err := collectIndex(context.Background(), stock, "0001", time.Now())
		Expect(err).To(BeNil())
		Expect(idx.OK).To(BeTrue())
		Expect(idx.Price).To(BeNumerically("~", 8565.06, 0.01))
		// prevClose = 8565.06 - (-549.49) = 9114.55
		Expect(idx.PrevClose).To(BeNumerically("~", 9114.55, 0.01))
		// tradingValue = 36_585_627 / 100 = 365856.27
		Expect(idx.TradingValue).To(BeNumerically("~", 365856.27, 0.1))
		Expect(idx.Advancers).To(Equal(87))
		Expect(idx.Decliners).To(Equal(823))
	})

	It("output 행 없으면 에러", func() {
		stock := testStock{
			indexResp: map[string]*auth.RESTResponse{
				"0001": {Body: map[string]any{}},
			},
		}
		_, err := collectIndex(context.Background(), stock, "0001", time.Now())
		Expect(err).NotTo(BeNil())
	})
})

// ──────────────────────────────────────────────────────────────────────────────
// market_test — buildWindow
// ──────────────────────────────────────────────────────────────────────────────

var _ = Describe("buildWindow", func() {
	nowBase := time.Date(2026, 6, 23, 13, 38, 0, 0, time.UTC)

	It("at-or-before 정확히 선택 — 1h/2h 구간 변동", func() {
		series := []yahoo.DailyClose{
			{DateUnix: nowBase.Add(-3 * time.Hour).Unix(), Close: 100},
			{DateUnix: nowBase.Add(-2 * time.Hour).Unix(), Close: 102},
			{DateUnix: nowBase.Add(-61 * time.Minute).Unix(), Close: 104},
			{DateUnix: nowBase.Add(-3 * time.Minute).Unix(), Close: 106},
		}
		quote := yahoo.Quote{Price: 107, ChangePercent: 0.5}
		win := buildWindow("^KS11", "KOSPI", quote, series, nowBase)
		Expect(win.OK).To(BeTrue())
		// 1h 기준: nowBase-1h=12:38, at-or-before → -61min 점 (Close=104)
		// 앵커 = lastTS = nowBase-3min
		// 1h 윈도우 target = lastTS-1h = nowBase-63min → at-or-before는 -2h 점(Close=102)
		Expect(win.Move1hPct).NotTo(BeNil())
		Expect(*win.Move1hPct).To(BeNumerically("~", (107.0-102.0)/102.0*100.0, 0.1))
		// 2h 윈도우 target = lastTS-2h = nowBase-123min → at-or-before는 -3h 점(Close=100)
		Expect(win.Move2hPct).NotTo(BeNil())
		Expect(*win.Move2hPct).To(BeNumerically("~", (107.0-100.0)/100.0*100.0, 0.1))
	})

	It("희소 시리즈(KRW=X): 45분 초과 간격 → Reason 설정", func() {
		series := []yahoo.DailyClose{
			// now-1h 기준의 목표 시각보다 70분 이전 점
			{DateUnix: nowBase.Add(-2*time.Hour - 10*time.Minute).Unix(), Close: 1530},
			{DateUnix: nowBase.Add(-5 * time.Minute).Unix(), Close: 1535},
		}
		quote := yahoo.Quote{Symbol: "KRW=X", Price: 1536, ChangePercent: 0.1}
		win := buildWindow("KRW=X", "원/달러", quote, series, nowBase)
		Expect(win.OK).To(BeTrue())
		Expect(win.Reason).NotTo(BeEmpty())
	})

	It("시리즈 없으면 Move nil, OK=true(현재가 있음)", func() {
		quote := yahoo.Quote{Price: 100, ChangePercent: 0}
		win := buildWindow("TEST", "테스트", quote, nil, nowBase)
		Expect(win.OK).To(BeTrue())
		Expect(win.Move1hPct).To(BeNil())
		Expect(win.Move2hPct).To(BeNil())
	})

	It("선물 날짜 경계: unix 기준으로만 계산 (달력 날짜 무관)", func() {
		// 자정을 가로지르는 시리즈
		midnight := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
		series := []yahoo.DailyClose{
			{DateUnix: midnight.Add(-2 * time.Hour).Unix(), Close: 19000}, // 전일 22:00
			{DateUnix: midnight.Add(-1 * time.Hour).Unix(), Close: 19100}, // 전일 23:00
			{DateUnix: midnight.Add(1 * time.Hour).Unix(), Close: 19200},  // 01:00
			{DateUnix: midnight.Add(2 * time.Hour).Unix(), Close: 19300},  // 02:00
		}
		queryNow := midnight.Add(2*time.Hour + 10*time.Minute)
		quote := yahoo.Quote{Price: 19350, ChangePercent: 1.5}
		win := buildWindow("NQ=F", "나스닥선물", quote, series, queryNow)
		Expect(win.OK).To(BeTrue())
		// 1h 기준: queryNow-1h=01:10 → at-or-before는 01:00 점(Close=19200)
		Expect(win.Move1hPct).NotTo(BeNil())
		Expect(*win.Move1hPct).To(BeNumerically("~", (19350.0-19200.0)/19200.0*100.0, 0.01))
	})
})
