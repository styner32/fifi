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
	// 1. 방향성 (Directionality)
	kospiDir := "KOSPI_FLAT"
	if p.KOSPI.Index.OK {
		pct := p.KOSPI.Index.ChangePct
		switch {
		case pct >= 1.5:
			kospiDir = "KOSPI_STRONG_UP"
		case pct >= 0.5:
			kospiDir = "KOSPI_UP"
		case pct > -0.5 && pct < 0.5:
			kospiDir = "KOSPI_WEAK_FLAT"
		case pct <= -1.5:
			kospiDir = "KOSPI_STRONG_DOWN"
		case pct <= -0.5:
			kospiDir = "KOSPI_WEAK"
		}
	}
	kosdaqDir := "KOSDAQ_FLAT"
	if p.KOSDAQ.Index.OK {
		pct := p.KOSDAQ.Index.ChangePct
		switch {
		case pct >= 1.5:
			kosdaqDir = "KOSDAQ_STRONG_UP"
		case pct >= 0.5:
			kosdaqDir = "KOSDAQ_UP"
		case pct > -0.5 && pct < 0.5:
			kosdaqDir = "KOSDAQ_WEAK_FLAT"
		case pct <= -1.5:
			kosdaqDir = "KOSDAQ_STRONG_DOWN"
		case pct <= -0.5:
			kosdaqDir = "KOSDAQ_WEAK"
		}
	}
	dirStr := fmt.Sprintf("%s · %s", kospiDir, kosdaqDir)

	// 2. 수급 (Flow)
	foreignNet := p.KOSPI.Flow.Foreign + p.KOSDAQ.Flow.Foreign
	instNet := p.KOSPI.Flow.Institution + p.KOSDAQ.Flow.Institution
	progNet := p.KOSPIProgram.Total + p.KOSDAQProgram.Total
	indivNet := p.KOSPI.Flow.Individual + p.KOSDAQ.Flow.Individual

	flowStatus := "NEUTRAL"
	flowDetail := ""
	if foreignNet < -100 && instNet < -100 {
		flowStatus = "NEGATIVE"
		if progNet < 0 {
			flowDetail = " (외국인·기관 및 프로그램 순매도)"
		} else {
			flowDetail = " (외국인·기관 순매도)"
		}
	} else if foreignNet > 100 && instNet > 100 {
		flowStatus = "POSITIVE"
		if progNet > 0 {
			flowDetail = " (외국인·기관 및 프로그램 순매수)"
		} else {
			flowDetail = " (외국인·기관 순매수)"
		}
	} else if foreignNet < -100 && instNet > 100 {
		flowStatus = "MIXED"
		flowDetail = " (기관 순매수 vs 외국인 순매도)"
	} else if foreignNet > 100 && instNet < -100 {
		flowStatus = "MIXED"
		flowDetail = " (외국인 순매수 vs 기관 순매도)"
	} else if indivNet > 500 && (foreignNet < 0 || instNet < 0) {
		flowStatus = "NEGATIVE"
		flowDetail = " (개인 순매수 vs 외인·기관 매도)"
	}
	flowStr := fmt.Sprintf("%s%s", flowStatus, flowDetail)

	// 3. 내부폭 (Breadth)
	breadthStr := "MIXED"
	totAdv := p.KOSPI.Index.Advancers + p.KOSDAQ.Index.Advancers + p.KOSPI.Index.UpperLimit + p.KOSDAQ.Index.UpperLimit
	totAll := p.KOSPI.Index.TotalCount + p.KOSDAQ.Index.TotalCount
	if totAll == 0 {
		totAll = p.KOSPI.Index.Advancers + p.KOSPI.Index.Decliners + p.KOSDAQ.Index.Advancers + p.KOSDAQ.Index.Decliners
	}
	if totAll > 0 {
		ratio := float64(totAdv) / float64(totAll)
		switch {
		case ratio >= 0.70:
			breadthStr = "STRONG"
		case ratio >= 0.55:
			breadthStr = "MODERATE_STRONG"
		case ratio <= 0.15:
			breadthStr = "EXTREME_WEAK"
		case ratio <= 0.35:
			breadthStr = "WEAK"
		default:
			breadthStr = "MIXED"
		}
	}

	// 4. 스트레스 (Stress)
	stressStr := "NORMAL"
	if p.VKOSPI.OK {
		if p.VKOSPI.Value >= 40.0 {
			stressStr = "PROVISIONAL_ELEVATED"
		} else if p.VKOSPI.Value >= 25.0 {
			stressStr = "ELEVATED"
		} else if p.VKOSPI.Value < 20.0 {
			stressStr = "NORMAL"
		}
	} else {
		stressStr = "PROVISIONAL_NORMAL"
	}
	for _, d := range p.Safety.Devices {
		if strings.HasPrefix(d.Device, "CB") && (d.State == "TRIGGERED" || d.State == "RELEASED") {
			stressStr = "EXTREME"
		} else if strings.HasPrefix(d.Device, "SIDECAR_") && (d.State == "TRIGGERED" || d.State == "RELEASED") {
			stressStr = "VERY_HIGH"
		}
	}

	// 5. 매크로 (Macro)
	macroStr := "MIXED"
	var adverseCount, supportiveCount int
	for _, w := range p.Macro {
		if w.Move1hPct != nil {
			if w.Symbol == "NQ=F" || w.Symbol == "ES=F" {
				if *w.Move1hPct <= -0.2 {
					adverseCount++
				} else if *w.Move1hPct >= 0.2 {
					supportiveCount++
				}
			}
			if w.Symbol == "CL=F" && math.Abs(*w.Move1hPct) >= 2.0 {
				adverseCount++
			}
			if w.Symbol == "^TNX" && *w.Move1hPct >= 1.5 {
				adverseCount++
			}
		}
	}
	if p.USDKRW.Move1hPct != nil {
		if *p.USDKRW.Move1hPct >= 0.1 {
			adverseCount++
		} else if *p.USDKRW.Move1hPct <= -0.1 {
			supportiveCount++
		}
	}
	if adverseCount >= 2 && supportiveCount == 0 {
		macroStr = "ADVERSE"
	} else if supportiveCount >= 2 && adverseCount == 0 {
		macroStr = "SUPPORTIVE"
	} else {
		macroStr = "MIXED"
	}

	// 6. 신뢰도 (Confidence)
	confidenceStr := "NOT_EVALUATED"
	isEvaluated := p.KOSPI.IntradayWin.Move1hPct != nil && p.KOSPI.FlowDelta1h != nil
	if isEvaluated {
		confidenceStr = fmt.Sprintf("EVALUATED (%.1f%%)", p.Assessment.Confidence)
	}

	// 7. 종합 (Composite)
	compositeStr := "COMPOSITE_NOT_CIRCULABLE"
	if isEvaluated {
		score := 0
		if *p.KOSPI.IntradayWin.Move1hPct >= 0.5 {
			score++
		} else if *p.KOSPI.IntradayWin.Move1hPct <= -0.5 {
			score--
		}
		if p.KOSPI.FlowDelta1h.Foreign < -500 {
			score -= 2
		} else if p.KOSPI.FlowDelta1h.Foreign > 500 {
			score++
		}
		if score >= 2 {
			compositeStr = "RISK_ON"
		} else if score <= -2 {
			compositeStr = "RISK_OFF"
		} else {
			compositeStr = "NEUTRAL"
		}
	}

	return fmt.Sprintf("종합: %s\n    [방향성 %s\n     · 수급 %s\n     · 내부폭 %s\n     · 스트레스 %s\n     · 매크로 %s\n     · 신뢰도 %s]",
		compositeStr, dirStr, flowStr, breadthStr, stressStr, macroStr, confidenceStr)
}

func AssessPulse(p *Pulse) PulseAssessment {
	assess := PulseAssessment{
		Direction:       "NEUTRAL",
		Stress:          "NORMAL",
		InternalBreadth: "MIXED",
		ExternalMacro:   "NEUTRAL",
		Confidence:      100.0,
	}

	var reasons []string
	var excluded []string

	// 1. Excluded Inputs & Confidence Calculation
	if !p.KOSPI.Index.OK || p.KOSPI.Index.Freshness == "STALE" {
		excluded = append(excluded, "KOSPI_INDEX")
		assess.Confidence -= 30
	}
	if !p.KOSDAQ.Index.OK || p.KOSDAQ.Index.Freshness == "STALE" {
		excluded = append(excluded, "KOSDAQ_INDEX")
		assess.Confidence -= 30
	}

	kospiFlowOK := p.KOSPI.Flow.OK && p.KOSPI.FlowDelta1h != nil
	if !kospiFlowOK {
		excluded = append(excluded, "KOSPI_FLOW")
		assess.Confidence -= 10
	}
	kosdaqFlowOK := p.KOSDAQ.Flow.OK && p.KOSDAQ.FlowDelta1h != nil
	if !kosdaqFlowOK {
		excluded = append(excluded, "KOSDAQ_FLOW")
		assess.Confidence -= 10
	}

	hasNQ := false
	hasES := false
	for _, w := range p.Macro {
		if w.Symbol == "NQ=F" && w.Freshness != "STALE" && w.OK {
			hasNQ = true
		}
		if w.Symbol == "ES=F" && w.Freshness != "STALE" && w.OK {
			hasES = true
		}
	}
	if !hasNQ && !hasES {
		excluded = append(excluded, "US_FUTURES")
		assess.Confidence -= 15
	}

	if !p.VKOSPI.OK || p.VKOSPI.Freshness == "STALE" {
		excluded = append(excluded, "VKOSPI")
		assess.Confidence -= 10
	}

	if assess.Confidence < 0 {
		assess.Confidence = 0
	}
	assess.ExcludedInputs = excluded

	// 2. Direction Calculation
	dirPoints := 0
	if !contains(excluded, "KOSPI_INDEX") && p.KOSPI.IntradayWin.Move1hPct != nil {
		m1h := *p.KOSPI.IntradayWin.Move1hPct
		switch {
		case m1h >= 1.0:
			dirPoints += 2
			reasons = append(reasons, fmt.Sprintf("코스피 1h 급등 (%s)", format.Percent(m1h)))
		case m1h >= 0.5:
			dirPoints += 1
			reasons = append(reasons, fmt.Sprintf("코스피 1h 상승 (%s)", format.Percent(m1h)))
		case m1h <= -1.0:
			dirPoints -= 2
			reasons = append(reasons, fmt.Sprintf("코스피 1h 급락 (%s)", format.Percent(m1h)))
		case m1h <= -0.5:
			dirPoints -= 1
			reasons = append(reasons, fmt.Sprintf("코스피 1h 하락 (%s)", format.Percent(m1h)))
		}
	}
	if !contains(excluded, "KOSDAQ_INDEX") && p.KOSDAQ.IntradayWin.Move1hPct != nil {
		m1h := *p.KOSDAQ.IntradayWin.Move1hPct
		switch {
		case m1h >= 1.2:
			dirPoints += 2
			reasons = append(reasons, fmt.Sprintf("코스닥 1h 급등 (%s)", format.Percent(m1h)))
		case m1h >= 0.6:
			dirPoints += 1
			reasons = append(reasons, fmt.Sprintf("코스닥 1h 상승 (%s)", format.Percent(m1h)))
		case m1h <= -1.2:
			dirPoints -= 2
			reasons = append(reasons, fmt.Sprintf("코스닥 1h 급락 (%s)", format.Percent(m1h)))
		case m1h <= -0.6:
			dirPoints -= 1
			reasons = append(reasons, fmt.Sprintf("코스닥 1h 하락 (%s)", format.Percent(m1h)))
		}
	}

	// US Futures average
	nqMove := 0.0
	esMove := 0.0
	nqCount := 0
	for _, w := range p.Macro {
		if w.Symbol == "NQ=F" && w.Move1hPct != nil {
			nqMove = *w.Move1hPct
			nqCount++
		}
		if w.Symbol == "ES=F" && w.Move1hPct != nil {
			esMove = *w.Move1hPct
			nqCount++
		}
	}
	if nqCount > 0 {
		avgUS := (nqMove + esMove) / float64(nqCount)
		if avgUS >= 0.5 {
			dirPoints += 1
			reasons = append(reasons, fmt.Sprintf("미국선물 1h 상승평균 (%s)", format.Percent(avgUS)))
		} else if avgUS <= -0.5 {
			dirPoints -= 1
			reasons = append(reasons, fmt.Sprintf("미국선물 1h 하락평균 (%s)", format.Percent(avgUS)))
		}
	}

	switch {
	case dirPoints >= 3:
		assess.Direction = "STRONG_UP"
	case dirPoints >= 1:
		assess.Direction = "UP"
	case dirPoints <= -3:
		assess.Direction = "STRONG_DOWN"
	case dirPoints <= -1:
		assess.Direction = "DOWN"
	default:
		assess.Direction = "NEUTRAL"
	}

	// 3. Stress Calculation
	stressPoints := 0
	if !contains(excluded, "VKOSPI") && p.VKOSPI.OK {
		if p.VKOSPI.Value >= 30.0 {
			stressPoints += 2
			reasons = append(reasons, fmt.Sprintf("VKOSPI 고공행진 (%.2f)", p.VKOSPI.Value))
		} else if p.VKOSPI.Value >= 22.0 {
			stressPoints += 1
			reasons = append(reasons, fmt.Sprintf("VKOSPI 불안정 (%.2f)", p.VKOSPI.Value))
		}
	}

	// Substantial intraday moves
	if !contains(excluded, "KOSPI_INDEX") && p.KOSPI.IntradayWin.Move1hPct != nil {
		if *p.KOSPI.IntradayWin.Move1hPct <= -1.5 {
			stressPoints += 2
		} else if *p.KOSPI.IntradayWin.Move1hPct <= -0.8 {
			stressPoints += 1
		}
	}

	// Institutional & Foreign Net selling
	var netForeign1h float64
	if kospiFlowOK {
		netForeign1h += p.KOSPI.FlowDelta1h.Foreign
	}
	if kosdaqFlowOK {
		netForeign1h += p.KOSDAQ.FlowDelta1h.Foreign
	}
	if netForeign1h <= -500.0 { // 5000억 이상 순매도
		stressPoints += 1
		reasons = append(reasons, fmt.Sprintf("외국인 양시장 1h %s 순매도", format.EokArrow(netForeign1h)))
	}

	switch {
	case stressPoints >= 3:
		assess.Stress = "VERY_HIGH"
	case stressPoints >= 1:
		assess.Stress = "HIGH"
	case stressPoints == 0:
		assess.Stress = "NORMAL"
	default:
		assess.Stress = "LOW"
	}

	// ── Hard Guardrails ──
	sidecarActive := false
	cbActive := false

	for _, d := range p.Safety.Devices {
		if strings.HasPrefix(d.Device, "SIDECAR_") && (d.State == "TRIGGERED" || d.State == "RELEASED" || d.State == "CONDITION_OBSERVED") {
			sidecarActive = true
		}
		if strings.HasPrefix(d.Device, "CB") && (d.State == "TRIGGERED" || d.State == "RELEASED" || d.State == "CONDITION_OBSERVED") {
			cbActive = true
		}
	}

	kospiDrop8 := p.KOSPI.Index.OK && p.KOSPI.Index.ChangePct <= -8.0
	kosdaqDrop8 := p.KOSDAQ.Index.OK && p.KOSDAQ.Index.ChangePct <= -8.0
	kospiDrop15 := p.KOSPI.Index.OK && p.KOSPI.Index.ChangePct <= -15.0
	kosdaqDrop15 := p.KOSDAQ.Index.OK && p.KOSDAQ.Index.ChangePct <= -15.0

	if sidecarActive || kospiDrop8 || kosdaqDrop8 {
		assess.Stress = "VERY_HIGH"
		reasons = append(reasons, "사이드카 발동 또는 지수 -8% 돌파로 스트레스 VERY_HIGH 강제 설정")
	}
	if cbActive || kospiDrop15 || kosdaqDrop15 {
		assess.Stress = "EXTREME"
		reasons = append(reasons, "서킷브레이커 발동 또는 지수 -15% 돌파로 스트레스 EXTREME 강제 설정")
	}

	// 4. Internal Breadth
	kospiRatio := 0.5
	kosdaqRatio := 0.5
	hasKospiB := false
	hasKosdaqB := false

	if p.KOSPI.Index.OK && (p.KOSPI.Index.Advancers > 0 || p.KOSPI.Index.Decliners > 0) {
		kospiRatio = float64(p.KOSPI.Index.Advancers) / float64(p.KOSPI.Index.Advancers+p.KOSPI.Index.Decliners)
		hasKospiB = true
	}
	if p.KOSDAQ.Index.OK && (p.KOSDAQ.Index.Advancers > 0 || p.KOSDAQ.Index.Decliners > 0) {
		kosdaqRatio = float64(p.KOSDAQ.Index.Advancers) / float64(p.KOSDAQ.Index.Advancers+p.KOSDAQ.Index.Decliners)
		hasKosdaqB = true
	}

	if hasKospiB || hasKosdaqB {
		avgRatio := 0.0
		if hasKospiB && hasKosdaqB {
			avgRatio = (kospiRatio + kosdaqRatio) / 2.0
		} else if hasKospiB {
			avgRatio = kospiRatio
		} else {
			avgRatio = kosdaqRatio
		}

		switch {
		case avgRatio >= 0.70:
			assess.InternalBreadth = "STRONG"
		case avgRatio <= 0.15:
			assess.InternalBreadth = "EXTREME_WEAK"
		case avgRatio <= 0.30:
			assess.InternalBreadth = "WEAK"
		default:
			assess.InternalBreadth = "MIXED"
		}
	}

	// 5. External Macro
	adversePoints := 0
	if p.USDKRW.Move1hPct != nil && *p.USDKRW.Move1hPct >= 0.15 {
		adversePoints++
		reasons = append(reasons, fmt.Sprintf("원/달러 환율 1h %s 급등 (원화 약세)", format.Percent(*p.USDKRW.Move1hPct)))
	}
	for _, w := range p.Macro {
		if w.Symbol == "^TNX" && w.Move1hPct != nil && *w.Move1hPct >= 2.0 {
			adversePoints++
			reasons = append(reasons, fmt.Sprintf("미국채10Y 1h %s 급등 (금리 상승 압박)", format.Percent(*w.Move1hPct)))
		}
		if w.Symbol == "CL=F" && w.Move1hPct != nil && math.Abs(*w.Move1hPct) >= 2.0 {
			adversePoints++
			reasons = append(reasons, fmt.Sprintf("WTI원유 1h %s 급변동 (원자재 불안)", format.Percent(*w.Move1hPct)))
		}
		if w.Symbol == "^N225" && w.Move1hPct != nil && *w.Move1hPct <= -1.0 {
			adversePoints++
			reasons = append(reasons, fmt.Sprintf("닛케이225 1h %s 낙폭 과대", format.Percent(*w.Move1hPct)))
		}
	}

	if adversePoints >= 2 {
		assess.ExternalMacro = "ADVERSE"
	} else if adversePoints == 0 {
		assess.ExternalMacro = "SUPPORTIVE"
	} else {
		assess.ExternalMacro = "NEUTRAL"
	}

	assess.Reasons = reasons
	return assess
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
