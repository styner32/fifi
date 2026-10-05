package pulse

import (
	"fmt"
	"math"
	"strings"

	"github.com/fifi/internal/format"
)

// Analyze는 Pulse에서 결정적(rule-based) 한국어 분석 불릿 목록을 생성합니다.
// §6.3 규칙 참조.
func Analyze(p *Pulse) []string {
	// Mutate Pulse to set Assessment
	p.Assessment = AssessPulse(p)

	var bullets []string

	// 1. 종합 시장 상태 (가장 상단에 배치)
	bullets = append(bullets, analyzeRiskLabel(p))

	// 2. 지수 모멘텀
	bullets = append(bullets, analyzeIndexMomentum(p)...)

	// 3. 수급 주도 주체
	bullets = append(bullets, analyzeFlowLeader(p)...)

	// 4. 외국인 × 환율 연계
	bullets = append(bullets, analyzeForexLink(p)...)

	// 5. 미국선물 동조/디커플링
	bullets = append(bullets, analyzeFuturesSync(p)...)

	// 6. 금리/유가 보조
	bullets = append(bullets, analyzeMacroSignals(p)...)

	return bullets
}

func analyzeIndexMomentum(p *Pulse) []string {
	var out []string
	m1h := p.KOSPI.IntradayWin.Move1hPct
	if m1h == nil {
		return out
	}
	v := *m1h
	switch {
	case v <= -0.5:
		out = append(out, fmt.Sprintf("코스피 최근 1h %s 하방 모멘텀 우위", format.Percent(v)))
	case v >= 0.5:
		out = append(out, fmt.Sprintf("코스피 최근 1h %s 반등 시도", format.Percent(v)))
	default:
		out = append(out, fmt.Sprintf("코스피 최근 1h %s 횡보", format.Percent(v)))
	}
	return out
}

func analyzeFlowLeader(p *Pulse) []string {
	var out []string
	d1h := p.KOSPI.FlowDelta1h
	d2h := p.KOSPI.FlowDelta2h
	if d1h == nil {
		return out
	}

	check := func(name string, v1h float64, field func(*FlowDelta) float64) {
		if math.Abs(hourlyRate(v1h, d1h.Elapsed)) < 1000 {
			return
		}
		acc := FlowAcceleration(d1h, d2h, field)
		accStr := ""
		if acc != "" {
			accStr = " (" + acc + ")"
		}
		cumStr := ""
		switch name {
		case "외국인":
			if p.KOSPI.Flow.OK {
				cumStr = fmt.Sprintf(" (누적 %s)", format.EokArrow(p.KOSPI.Flow.Foreign))
			}
		case "기관":
			if p.KOSPI.Flow.OK {
				cumStr = fmt.Sprintf(" (누적 %s)", format.EokArrow(p.KOSPI.Flow.Institution))
			}
		case "개인":
			if p.KOSPI.Flow.OK {
				cumStr = fmt.Sprintf(" (누적 %s)", format.EokArrow(p.KOSPI.Flow.Individual))
			}
		}
		out = append(out, fmt.Sprintf("KOSPI %s 최근 %s %s %s%s%s", name, elapsedLabel(d1h.Elapsed), format.EokArrow(v1h), flowDirection(v1h), accStr, cumStr))
	}

	check("외국인", d1h.Foreign, func(d *FlowDelta) float64 { return d.Foreign })
	check("기관", d1h.Institution, func(d *FlowDelta) float64 { return d.Institution })
	check("개인", d1h.Individual, func(d *FlowDelta) float64 { return d.Individual })

	return out
}

func analyzeForexLink(p *Pulse) []string {
	var out []string
	d1h := p.KOSPI.FlowDelta1h
	if d1h == nil || !p.KOSPI.Flow.OK {
		return out
	}

	foreignSelling := hourlyRate(d1h.Foreign, d1h.Elapsed) < -500
	usdkrwRise := p.USDKRW.Move1hPct != nil && *p.USDKRW.Move1hPct > 0

	if foreignSelling && usdkrwRise {
		out = append(out, fmt.Sprintf("원화 약세(원/달러 1h %s) 동반 외국인 이탈 → 환차손 회피성 매도 가능성", format.Percent(*p.USDKRW.Move1hPct)))
	} else if !usdkrwRise && p.USDKRW.Move1hPct != nil {
		out = append(out, fmt.Sprintf("원/달러 1h %s (원화 강보합) → 환율發 압력 제한적", format.Percent(*p.USDKRW.Move1hPct)))
	}
	return out
}

func analyzeFuturesSync(p *Pulse) []string {
	var out []string
	kospi1h := p.KOSPI.IntradayWin.Move1hPct
	var nqWin *Window
	for i := range p.Macro {
		if p.Macro[i].Symbol == "NQ=F" {
			nqWin = &p.Macro[i]
			break
		}
	}
	if kospi1h == nil || nqWin == nil || nqWin.Move1hPct == nil {
		return out
	}

	nq1h := *nqWin.Move1hPct
	ksp1h := *kospi1h

	if sign(nq1h) == sign(ksp1h) {
		out = append(out, fmt.Sprintf("나스닥선물 1h %s — 코스피와 동조", format.Percent(nq1h)))
	} else {
		out = append(out, fmt.Sprintf(
			"디커플링: 나스닥선물 1h %s vs 코스피 1h %s — 갭 메우기 여지",
			format.Percent(nq1h), format.Percent(ksp1h),
		))
	}
	return out
}

func analyzeMacroSignals(p *Pulse) []string {
	var out []string
	for i := range p.Macro {
		w := &p.Macro[i]
		if w.Move1hPct == nil {
			continue
		}
		v := *w.Move1hPct
		switch w.Symbol {
		case "^TNX":
			if v >= 1.5 {
				out = append(out, fmt.Sprintf("미국채10Y 1h %s 급등 → 금리 상승이 위험자산 부담", format.Percent(v)))
			} else if v <= -1.5 {
				out = append(out, fmt.Sprintf("미국채10Y 1h %s 하락 → 금리 하락, 위험자산 긍정적", format.Percent(v)))
			}
		case "CL=F":
			if math.Abs(v) >= 1.5 {
				dir := "급등"
				if v < 0 {
					dir = "급락"
				}
				out = append(out, fmt.Sprintf("WTI원유 1h %s %s → 에너지·인플레이션 주의", format.Percent(v), dir))
			}
		}
	}
	return out
}

// analyzeRiskLabel은 신호들을 종합해 구조화된 시장 상태 블록을 반환합니다.
func analyzeRiskLabel(p *Pulse) string {
	a := p.Assessment
	return fmt.Sprintf("종합: COMPOSITE_NOT_CIRCULABLE\n    [방향성 %s · 내부폭 %s\n     · 스트레스 %s · 매크로 %s\n     · 신뢰도 NOT_EVALUATED (예측 정확도 검증 없음)]", a.Direction, a.InternalBreadth, a.Stress, a.ExternalMacro)
}

func AssessPulse(p *Pulse) PulseAssessment {
	a := PulseAssessment{Direction: "NOT_EVALUATED", Stress: "NOT_EVALUATED", InternalBreadth: "NOT_EVALUATED", ExternalMacro: "NOT_EVALUATED"}
	k, q := p.KOSPI, p.KOSDAQ
	validWindow := func(w Window) bool { return w.OK && usableFreshness(w.Freshness) && w.Move1hPct != nil }
	for _, m := range []Market{k, q} {
		if !m.Index.OK || !usableFreshness(m.Index.Freshness) {
			a.ExcludedInputs = append(a.ExcludedInputs, m.Name+"_INDEX_TIME")
		}
		if m.FlowDelta1h == nil {
			a.ExcludedInputs = append(a.ExcludedInputs, m.Name+"_FLOW_1H")
		}
	}
	if validWindow(k.IntradayWin) && validWindow(q.IntradayWin) {
		v := (*k.IntradayWin.Move1hPct + *q.IntradayWin.Move1hPct) / 2
		a.Direction = "NEUTRAL"
		if v >= 0.5 {
			a.Direction = "UP"
		}
		if v <= -0.5 {
			a.Direction = "DOWN"
		}
	}
	a.ExcludedInputs = append(a.ExcludedInputs, "BREADTH_LIMIT_COUNT_INCLUSION_UNVERIFIED")
	if p.VKOSPI.OK && usableFreshness(p.VKOSPI.Freshness) {
		a.Stress = "NORMAL"
		if p.VKOSPI.Value >= 22 {
			a.Stress = "HIGH"
		}
		if p.VKOSPI.Value >= 30 {
			a.Stress = "VERY_HIGH"
		}
	} else {
		a.ExcludedInputs = append(a.ExcludedInputs, "VKOSPI_TIME")
	}
	for _, d := range p.Safety.Devices {
		if d.Verification == "OFFICIAL" && (d.State == "TRIGGERED" || d.State == "RELEASED") {
			if strings.HasPrefix(d.Device, "CB") {
				a.Stress = "EXTREME"
			} else if a.Stress != "EXTREME" {
				a.Stress = "VERY_HIGH"
			}
		}
	}
	// Missing adverse observations are never positive evidence.
	macro := map[string]Window{}
	for _, w := range p.Macro {
		macro[w.Symbol] = w
	}
	macro["KRW=X"] = p.USDKRW
	complete := true
	for _, sym := range []string{"NQ=F", "ES=F", "KRW=X", "CL=F", "^TNX"} {
		if !validWindow(macro[sym]) {
			complete = false
			a.ExcludedInputs = append(a.ExcludedInputs, sym+"_1H")
		}
	}
	if complete {
		nq, es, fx, oil, yield := *macro["NQ=F"].Move1hPct, *macro["ES=F"].Move1hPct, *macro["KRW=X"].Move1hPct, *macro["CL=F"].Move1hPct, *macro["^TNX"].Move1hPct
		a.ExternalMacro = "MIXED"
		if nq < -0.2 && es < -0.2 || fx >= .15 || math.Abs(oil) >= 2 || yield >= 2 {
			a.ExternalMacro = "ADVERSE"
		} else if nq >= .2 && es >= .2 && fx <= 0 {
			a.ExternalMacro = "SUPPORTIVE"
		}
	}
	a.Reasons = []string{"관측 시각과 비교 기준을 검증한 입력만 평가; 종합 및 예측 신뢰도는 미검증"}
	return a
}

func contains(arr []string, val string) bool {
	for _, x := range arr {
		if x == val {
			return true
		}
	}
	return false
}

func sign(v float64) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}
