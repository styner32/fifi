package pulse

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Analyze", func() {
	base := time.Date(2026, 6, 23, 13, 38, 0, 0, kstLocation)
	nq1hUp := 0.7
	nq1hDown := -0.5

	makeNQWin := func(move1h float64) Window {
		return Window{Symbol: "NQ=F", Label: "나스닥100선물", OK: true, Move1hPct: &move1h}
	}

	It("외인 매도 + 원화 약세 → 환차손 회피성 코멘트", func() {
		usdkrwMove := 0.3
		p := &Pulse{
			Now:  base,
			Date: "20260623",
			KOSPI: Market{
				Index:       IndexLevel{Price: 8565.06, OK: true},
				IntradayWin: Window{OK: true, Move1hPct: ptr(-1.8)},
				Flow:        FlowSnapshot{OK: true, Foreign: -35560},
				FlowDelta1h: &FlowDelta{Elapsed: 60, Foreign: -12000, Institution: -4000, Individual: 15500},
				FlowDelta2h: &FlowDelta{Elapsed: 120, Foreign: -8000, Institution: -2000, Individual: 10000},
			},
			USDKRW: Window{OK: true, Current: 1534.43, Move1hPct: &usdkrwMove},
			Macro:  []Window{makeNQWin(nq1hUp)},
			Errors: map[string]string{},
		}
		bullets := Analyze(p)
		combined := ""
		for _, b := range bullets {
			combined += b + "\n"
		}
		Expect(combined).To(ContainSubstring("환차손"))
		Expect(combined).To(ContainSubstring("외국인"))
	})

	It("미선물 동조: KOSPI↓ + NQ↓ → 동조 코멘트", func() {
		p := &Pulse{
			Now:  base,
			Date: "20260623",
			KOSPI: Market{
				Index:       IndexLevel{OK: true},
				IntradayWin: Window{OK: true, Move1hPct: ptr(-1.5)},
				Flow:        FlowSnapshot{OK: true},
			},
			USDKRW: Window{OK: true},
			Macro:  []Window{makeNQWin(nq1hDown)},
			Errors: map[string]string{},
		}
		bullets := Analyze(p)
		combined := ""
		for _, b := range bullets {
			combined += b + "\n"
		}
		Expect(combined).To(ContainSubstring("동조"))
	})

	It("미선물 디커플링: KOSPI↓ + NQ↑ → 디커플링 코멘트", func() {
		p := &Pulse{
			Now:  base,
			Date: "20260623",
			KOSPI: Market{
				Index:       IndexLevel{OK: true},
				IntradayWin: Window{OK: true, Move1hPct: ptr(-1.5)},
				Flow:        FlowSnapshot{OK: true},
			},
			USDKRW: Window{OK: true},
			Macro:  []Window{makeNQWin(nq1hUp)},
			Errors: map[string]string{},
		}
		bullets := Analyze(p)
		combined := ""
		for _, b := range bullets {
			combined += b + "\n"
		}
		Expect(combined).To(ContainSubstring("디커플링"))
	})

	It("Risk-off 종합 라벨: 외인 대규모 매도 + 원화 약세", func() {
		usdkrwMove := 0.5
		p := &Pulse{
			Now:  base,
			Date: "20260623",
			KOSPI: Market{
				Index:       IndexLevel{OK: true},
				IntradayWin: Window{OK: true, Move1hPct: ptr(-1.0)},
				Flow:        FlowSnapshot{OK: true},
				FlowDelta1h: &FlowDelta{Elapsed: 60, Foreign: -15000},
			},
			USDKRW: Window{OK: true, Move1hPct: &usdkrwMove},
			Macro:  []Window{makeNQWin(0.3)},
			Errors: map[string]string{},
		}
		bullets := Analyze(p)
		Expect(bullets[0]).To(ContainSubstring("RISK_OFF"))
	})

	It("데이터 없을 때 패닉 없이 실행", func() {
		p := &Pulse{
			Now:    base,
			Date:   "20260623",
			Errors: map[string]string{},
		}
		Expect(func() { Analyze(p) }).NotTo(Panic())
	})
})
