package pulse

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fifi/internal/format"
)

const kstLayout = "2006-01-02 15:04 KST"

// Render는 Pulse를 한국어 리포트 문자열로 렌더링합니다.
func Render(p *Pulse) string {
	var b strings.Builder
	nowKST := p.Now.In(kstLocation)

	// ── 헤더 및 시장 세션 상태 ───────────────────────────────────────────────
	storeInfo := ""
	storeDir := p.StoreDir
	if storeDir == "" {
		storeDir = ".cache/pulse"
	}
	if p.StoredCount > 0 && p.PrevTS != nil {
		prevKST := p.PrevTS.In(kstLocation)
		storeInfo = fmt.Sprintf("당일 적립 %d회 · 직전 %s", p.StoredCount, prevKST.Format("2006-01-02 15:04:05 KST"))
	} else if p.StoredCount > 0 {
		storeInfo = fmt.Sprintf("당일 적립 %d회", p.StoredCount)
	} else {
		storeInfo = "비교 이력 없음 (1h/2h 수급 변화 N/A)"
	}

	isHoliday := IsHoliday("KRX", nowKST.Format("20060102"))
	phase := GetMarketPhase("KRX", p.Now, isHoliday)

	b.WriteString(fmt.Sprintf("🫀 시장 펄스  %s   (%s)\n",
		nowKST.Format(kstLayout), storeInfo))
	b.WriteString(fmt.Sprintf("📊 국내 거래소 세션 상태: %s\n", phase))
	if phase == "CLOSING_AUCTION" {
		b.WriteString("⚠️  장 마감 동시호가 진행 중 (15:20 ~ 15:30) · 실시간 지수 변동은 체결 분봉 생성 시점까지 지연될 수 있습니다.\n")
	}
	b.WriteString("\n")

	// ── 1. 지수 ───────────────────────────────────────────────────────────────
	b.WriteString("📈 지수 (전일대비 · 최근1h/2h)\n")
	renderMarketIndex(&b, p.KOSPI)
	renderMarketIndex(&b, p.KOSDAQ)
	b.WriteString("\n")

	// ── 2. 시장 안전장치 ───────────────────────────────────────────────────
	b.WriteString("🚨 시장 안전장치 상태 및 근접도 (관측과 공식 확인 구분)\n")
	renderMarketSafety(&b, p.Safety)
	b.WriteString("\n")

	// ── 3. 수급 ───────────────────────────────────────────────────────────────
	b.WriteString("💰 수급 현황 (누적 및 델타 분리)\n")
	renderMarketFlow(&b, p.KOSPI)
	renderMarketFlow(&b, p.KOSDAQ)
	b.WriteString("\n")

	// ── 4. 프로그램매매 ─────────────────────────────────────────────────────
	b.WriteString("🧮 프로그램매매 (차익 / 비차익 / 합계)\n")
	renderProgramTrade(&b, "KOSPI", p.KOSPIProgram, p.KOSPIProgramDeltaPrev, p.KOSPIProgramDeltaAnchor, p.KOSPIProgramDelta1h, p.KOSPIProgramDelta2h)
	renderProgramTrade(&b, "KOSDAQ", p.KOSDAQProgram, p.KOSDAQProgramDeltaPrev, p.KOSDAQProgramDeltaAnchor, p.KOSDAQProgramDelta1h, p.KOSDAQProgramDelta2h)
	b.WriteString("\n")

	// ── 5. KOSPI 기여도 ─────────────────────────────────────────────────────
	b.WriteString("🧱 KOSPI 편입표시·보통주 상위 10종목 참고 (KIS 마스터 기준)\n")
	b.WriteString("  지수 영향·기여율: NOT_EVALUATED — 공식 구성종목·산출주식수·가중치 기준일 및 시세 정합성 미검증\n")
	if len(p.Contributions) == 0 {
		b.WriteString("  검증 가능한 종목별 참고 자료 없음\n")
	} else {
		c := p.Contributions[0]
		b.WriteString(fmt.Sprintf("  마스터 %s · %d종목 · 동일 집합 시가총액 분모 %.0f억원 · %s\n", c.MasterDate, c.UniverseCount, c.Denominator, c.WeightStatus))
		for _, c := range p.Contributions {
			b.WriteString(fmt.Sprintf("  %-10s %-6s 등락 %+.2f%% · 마스터 시총비중 %.2f%%\n", c.Name, c.Code, c.ChangePct, c.WeightPct))
		}
	}
	b.WriteString("\n")

	// ── 6. 국내 파생·변동성 ─────────────────────────────────────────────────
	b.WriteString("🌡️ 국내 파생·변동성\n")
	renderDomesticDerivatives(&b, p)
	b.WriteString("\n")

	// ── 7. 환율 ───────────────────────────────────────────────────────────────
	b.WriteString("💱 환율\n")
	renderWindowLine(&b, "  원/달러 (Yahoo KRW=X)", p.USDKRW)
	b.WriteString("\n")

	// ── 8. 미국선물·매크로 ────────────────────────────────────────────────────
	b.WriteString("🌐 미국선물·매크로 (최근1h/2h)\n")
	for _, w := range p.Macro {
		if w.Symbol == "^TNX" {
			renderYieldLine(&b, "  "+w.Label, w)
		} else {
			renderWindowLine(&b, "  "+w.Label, w)
		}
	}
	b.WriteString("\n")

	// ── 9. 시장 상태 ──────────────────────────────────────────────────────────
	b.WriteString("🧭 시장 상태\n")
	if len(p.Analysis) == 0 {
		b.WriteString("  데이터 수집 중\n")
	}
	for _, bullet := range p.Analysis {
		b.WriteString("  • " + bullet + "\n")
	}
	b.WriteString("\n")

	// ── 10. 저장 정보 ─────────────────────────────────────────────────────────
	if len(p.Errors) > 0 {
		b.WriteString("⚠️  오류\n")
		for k, v := range p.Errors {
			b.WriteString(fmt.Sprintf("  [%s] %s\n", k, v))
		}
		b.WriteString("\n")
	}

	if p.Saved {
		b.WriteString(fmt.Sprintf("💾 %s/pulse_%s.jsonl (+1행) · pulse_%s.md 갱신\n", storeDir, p.Date, p.Date))
	} else {
		b.WriteString(fmt.Sprintf("💾 저장 안 함 · 대상 경로 %s/pulse_%s.jsonl\n", storeDir, p.Date))
	}

	return b.String()
}

func renderMarketIndex(b *strings.Builder, m Market) {
	idx := m.Index
	if !idx.OK {
		b.WriteString(fmt.Sprintf("  %-7s  데이터 없음\n", m.Name))
		return
	}

	b.WriteString(fmt.Sprintf("  %s · %s\n", idx.Source, observationLabel(idx.LastTS, idx.FetchedAt, idx.Freshness)))
	win1h := fmtPctPtr(m.IntradayWin.Move1hPct)
	win2h := fmtPctPtr(m.IntradayWin.Move2hPct)

	tradingStr := ""
	if idx.TradingValue > 0 {
		tradingStr = fmt.Sprintf("   거래대금 %s", fmtAmountEok(idx.TradingValue))
	}

	b.WriteString(fmt.Sprintf("  %-7s %9.2f  %s%s%%   1h %s  2h %s%s\n",
		m.Name, idx.Price,
		arrowNeutral(idx.ChangePct), fmt.Sprintf("%.2f", idx.ChangePct),
		win1h, win2h,
		tradingStr,
	))

	if idx.Open > 0 {
		b.WriteString(fmt.Sprintf("          시 %.2f / 고 %s / 저 %s",
			idx.Open, positivePrice(idx.High), positivePrice(idx.Low)))
		total := idx.TotalCount
		if total == 0 {
			total = idx.UpperLimit + idx.Advancers + idx.Unchanged + idx.Decliners + idx.LowerLimit
		}
		if idx.BreadthOK && total > 0 {
			uplmStr := ""
			if idx.UpperLimit > 0 {
				uplmStr = fmt.Sprintf(" (상한 %d)", idx.UpperLimit)
			}
			lslmStr := ""
			if idx.LowerLimit > 0 {
				lslmStr = fmt.Sprintf(" (하한 %d)", idx.LowerLimit)
			}
			b.WriteString(fmt.Sprintf("   상승 %d%s · 보합 %d · 하락 %d%s · 응답 분류합 %d (상·하한 포함관계 미확인)",
				idx.Advancers, uplmStr, idx.Unchanged, idx.Decliners, lslmStr, total))
		} else if idx.BreadthOK && idx.Advancers+idx.Decliners > 0 {
			b.WriteString(fmt.Sprintf("   상승 %d · 하락 %d", idx.Advancers, idx.Decliners))
		}
		b.WriteString("\n")
	}
}

func renderMarketFlow(b *strings.Builder, m Market) {
	flow := m.Flow
	if !flow.OK {
		b.WriteString(fmt.Sprintf("  %-7s  수급 데이터 없음\n", m.Name))
		return
	}

	b.WriteString(fmt.Sprintf("  %s · %s\n", flow.Source, observationLabel(flow.LastTS, flow.FetchedAt, "UNKNOWN")))
	etcFrgnStr := " · 기타외국인 N/A"
	if flow.EtcForeignOK {
		etcFrgnStr = ""
	}
	if flow.EtcForeignOK && math.Abs(flow.EtcForeign) > 0.001 {
		etcFrgnStr = fmt.Sprintf(" · 기타외국인 %s", fmtEok(flow.EtcForeign))
	}

	total := flow.Foreign + flow.Institution + flow.Individual + flow.EtcCorp + flow.EtcForeign
	sumStr := "(응답 주체 소계 · 전체 합계 확인 불가)"
	if flow.EtcForeignOK {
		sumStr = "(합계 0억)"
	}
	if flow.EtcForeignOK && math.Abs(total) >= 1.0 {
		sumStr = fmt.Sprintf("(⚠️ 합계 불일치: %s)", fmtEok(total))
	}

	b.WriteString(fmt.Sprintf("  %-7s  누적: 외국인 %s · 기관 %s · 개인 %s · 기타법인 %s%s %s\n",
		m.Name, fmtEok(flow.Foreign), fmtEok(flow.Institution), fmtEok(flow.Individual), fmtEok(flow.EtcCorp), etcFrgnStr, sumStr))

	if m.FlowDeltaPrev != nil {
		prevEtcFrgn := ""
		if math.Abs(m.FlowDeltaPrev.EtcForeign) > 0.001 {
			prevEtcFrgn = fmt.Sprintf(" · 기타외국인 %s", fmtEok(m.FlowDeltaPrev.EtcForeign))
		}
		prevTotal := m.FlowDeltaPrev.Foreign + m.FlowDeltaPrev.Institution + m.FlowDeltaPrev.Individual + m.FlowDeltaPrev.EtcCorp + m.FlowDeltaPrev.EtcForeign
		prevWarn := ""
		if math.Abs(prevTotal) >= 1.0 {
			prevWarn = fmt.Sprintf(" ⚠️ 불일치(%s)", fmtEok(prevTotal))
		}
		b.WriteString(fmt.Sprintf("           직전대비: 외국인 %s · 기관 %s · 개인 %s · 기타법인 %s%s%s  (경과 %.1f분)\n",
			fmtEok(m.FlowDeltaPrev.Foreign), fmtEok(m.FlowDeltaPrev.Institution), fmtEok(m.FlowDeltaPrev.Individual), fmtEok(m.FlowDeltaPrev.EtcCorp), prevEtcFrgn, prevWarn, m.FlowDeltaPrev.Elapsed))
	}
	if m.FlowDeltaAnchor != nil {
		anchorEtcFrgn := ""
		if math.Abs(m.FlowDeltaAnchor.EtcForeign) > 0.001 {
			anchorEtcFrgn = fmt.Sprintf(" · 기타외국인 %s", fmtEok(m.FlowDeltaAnchor.EtcForeign))
		}
		anchorTotal := m.FlowDeltaAnchor.Foreign + m.FlowDeltaAnchor.Institution + m.FlowDeltaAnchor.Individual + m.FlowDeltaAnchor.EtcCorp + m.FlowDeltaAnchor.EtcForeign
		anchorWarn := ""
		if math.Abs(anchorTotal) >= 1.0 {
			anchorWarn = fmt.Sprintf(" ⚠️ 불일치(%s)", fmtEok(anchorTotal))
		}
		b.WriteString(fmt.Sprintf("           당일 첫 수집 대비: 외국인 %s · 기관 %s · 개인 %s · 기타법인 %s%s%s  (경과 %.1f분)\n",
			fmtEok(m.FlowDeltaAnchor.Foreign), fmtEok(m.FlowDeltaAnchor.Institution), fmtEok(m.FlowDeltaAnchor.Individual), fmtEok(m.FlowDeltaAnchor.EtcCorp), anchorEtcFrgn, anchorWarn, m.FlowDeltaAnchor.Elapsed))
	}
	if m.FlowDelta1h != nil {
		acc := FlowAcceleration(m.FlowDelta1h, m.FlowDelta2h, func(d *FlowDelta) float64 { return d.Foreign })
		accStr := ""
		if acc != "" {
			accStr = " (" + acc + ")"
		}
		d1hEtcFrgn := ""
		if math.Abs(m.FlowDelta1h.EtcForeign) > 0.001 {
			d1hEtcFrgn = fmt.Sprintf(" · 기타외국인 %s", fmtEok(m.FlowDelta1h.EtcForeign))
		}
		d1hTotal := m.FlowDelta1h.Foreign + m.FlowDelta1h.Institution + m.FlowDelta1h.Individual + m.FlowDelta1h.EtcCorp + m.FlowDelta1h.EtcForeign
		d1hWarn := ""
		if math.Abs(d1hTotal) >= 1.0 {
			d1hWarn = fmt.Sprintf(" ⚠️ 불일치(%s)", fmtEok(d1hTotal))
		}
		b.WriteString(fmt.Sprintf("           최근1h: 외국인 %s%s · 기관 %s · 개인 %s · 기타법인 %s%s%s\n",
			fmtEok(m.FlowDelta1h.Foreign), accStr, fmtEok(m.FlowDelta1h.Institution), fmtEok(m.FlowDelta1h.Individual), fmtEok(m.FlowDelta1h.EtcCorp), d1hEtcFrgn, d1hWarn))
	}

	renderFlowDetail(b, flow)
}

func renderProgramTrade(b *strings.Builder, market string, cur ProgramTradeSnapshot, prev, anchor, d1h, d2h *ProgramTradeDelta) {
	if !cur.OK {
		b.WriteString(fmt.Sprintf("  %-7s 데이터 없음\n", market))
		return
	}
	b.WriteString(fmt.Sprintf("  %-7s 누적: 차익 %s / 비차익 %s / 합계 %s\n", market,
		fmtEok(cur.Arbitrage), fmtEok(cur.NonArbitrage), fmtEok(cur.Total)))
	if prev != nil {
		b.WriteString(fmt.Sprintf("           직전대비: 차익 %s / 비차익 %s / 합계 %s  (경과 %.1f분)\n",
			fmtEok(prev.Arbitrage), fmtEok(prev.NonArbitrage), fmtEok(prev.Total), prev.Elapsed))
	}
	if anchor != nil {
		b.WriteString(fmt.Sprintf("           당일 첫 수집 대비: 차익 %s / 비차익 %s / 합계 %s  (경과 %.1f분)\n",
			fmtEok(anchor.Arbitrage), fmtEok(anchor.NonArbitrage), fmtEok(anchor.Total), anchor.Elapsed))
	}
	if d1h != nil {
		b.WriteString(fmt.Sprintf("           최근1h: 차익 %s / 비차익 %s / 합계 %s\n",
			fmtEok(d1h.Arbitrage), fmtEok(d1h.NonArbitrage), fmtEok(d1h.Total)))
	}
	if cur.AsOf == "close" {
		b.WriteString("           └ 장마감 적용\n")
	} else if len(cur.AsOf) >= 4 {
		b.WriteString(fmt.Sprintf("           └ 응답 집계시각 %s:%s (관측 날짜 미확인)\n", cur.AsOf[:2], cur.AsOf[2:4]))
	}
}

func renderMarketSafety(b *strings.Builder, safety MarketSafety) {
	if len(safety.Devices) == 0 {
		b.WriteString("  장치 데이터 없음\n")
		return
	}
	for _, d := range safety.Devices {
		distStr := "N/A"
		if d.FuturesGapPct != nil && d.SpotGapPct != nil {
			distStr = fmt.Sprintf("선물 %.2f%%p / 현물 %.2f%%p", *d.FuturesGapPct, *d.SpotGapPct)
		} else if d.ThresholdDistancePct != nil {
			distStr = fmt.Sprintf("%.2f%%p", *d.ThresholdDistancePct)
		}

		rateInfo := ""
		if d.FuturesChangePct != nil && d.SpotChangePct != nil && d.SpotThreshold != nil {
			// KOSDAQ Sidecar
			fSign := "+"
			sSign := "+"
			if d.Device == "SIDECAR_SELL" {
				fSign = "-"
				sSign = "-"
			}
			rateInfo = fmt.Sprintf("(선물 %+.2f%% [임계 %s%.1f%%] · 현물 %+.2f%% [임계 %s%.1f%%])",
				*d.FuturesChangePct, fSign, d.Threshold, *d.SpotChangePct, sSign, *d.SpotThreshold)
		} else if d.FuturesChangePct != nil {
			// KOSPI Sidecar
			fSign := "+"
			if d.Device == "SIDECAR_SELL" {
				fSign = "-"
			}
			rateInfo = fmt.Sprintf("(선물 %+.2f%% [임계 %s%.1f%%])", *d.FuturesChangePct, fSign, d.Threshold)
		} else if d.IndexChangePct != nil {
			// CB
			rateInfo = fmt.Sprintf("(지수 %+.2f%% [임계 -%.1f%%])", *d.IndexChangePct, d.Threshold)
		} else {
			rateInfo = fmt.Sprintf("(임계 %.1f%%)", d.Threshold)
		}

		statusDetail := d.EligibilityReason
		if d.State == "TRIGGERED" {
			statusDetail = fmt.Sprintf("공식 발동 기록 (발동시각 %s · 확인된 해제시각 %s)", d.TriggeredAt, d.ReleasedAt)
		} else if d.State == "RELEASED" {
			statusDetail = fmt.Sprintf("발동 후 해제됨 (발동시각 %s)", d.TriggeredAt)
		} else if d.State == "CONDITION_OBSERVED" {
			statusDetail = fmt.Sprintf("조건 관측 충족 (관측시각 %s · 공식 확인 대기)", d.ConditionObservedAt)
		}

		b.WriteString(fmt.Sprintf("  [%s] %s %s: 상태 %s (간격 %s) · %s\n",
			d.Market, d.Device, rateInfo, d.State, distStr, statusDetail))
	}
}

// RenderSafetyOnly는 서킷브레이커와 사이드카 안전장치 현황만 렌더링합니다.
func RenderSafetyOnly(p *Pulse) string {
	var b strings.Builder
	nowKST := p.Now.In(kstLocation)

	b.WriteString(fmt.Sprintf("🚨 시장 안전장치 현황 (Sidecar / Circuit Breaker) — %s\n", nowKST.Format(kstLayout)))
	renderMarketSafety(&b, p.Safety)
	return b.String()
}

func renderDomesticDerivatives(b *strings.Builder, p *Pulse) {
	if p.KOSPI200Future.OK && p.KOSPI200Future.SpotOK {
		b.WriteString("  " + observationLabel(p.KOSPI200Future.LastTS, p.KOSPI200Future.FetchedAt, p.KOSPI200Future.Freshness) + "\n")
		regime := p.KOSPI200Future.Alignment
		b.WriteString(fmt.Sprintf("  KOSPI200 선물 %s  %.2f (%+.2f%%) · 현물 %.2f · 단순스프레드(raw_spread) %+.2fp (%s)\n",
			p.KOSPI200Future.Code, p.KOSPI200Future.Price, p.KOSPI200Future.ChangePct,
			p.KOSPI200Future.SpotPrice, p.KOSPI200Future.Basis, regime))
		if p.BasisDeltaPrev != nil {
			b.WriteString(fmt.Sprintf("           직전대비 스프레드 변동: %+.2fp\n", p.BasisDeltaPrev.Value))
		}
		if p.BasisDeltaAnchor != nil {
			b.WriteString(fmt.Sprintf("           당일 첫 수집 대비 스프레드 변동: %+.2fp\n", p.BasisDeltaAnchor.Value))
		}
		if p.BasisDelta1h != nil {
			b.WriteString(fmt.Sprintf("           최근1h 스프레드 변동: %+.2fp\n", p.BasisDelta1h.Value))
		}
	} else {
		b.WriteString("  KOSPI200 선물 데이터 없음\n")
	}
	if p.VKOSPI.OK {
		b.WriteString("  " + observationLabel(p.VKOSPI.LastTS, p.VKOSPI.FetchedAt, p.VKOSPI.Freshness) + "\n")
		b.WriteString(fmt.Sprintf("  VKOSPI %.2f  전일 %+.2f%% · %s\n", p.VKOSPI.Value, p.VKOSPI.ChangePct, p.VKOSPI.Source))
	} else {
		b.WriteString("  VKOSPI 데이터 없음\n")
	}
}

func renderFlowDetail(b *strings.Builder, flow FlowSnapshot) {
	instSum := flow.FinInvest + flow.Insurance + flow.InvTrust + flow.EtcFin + flow.Bank + flow.Pension + flow.PrivEquity
	instDiff := flow.Institution - instSum

	sumStr := fmt.Sprintf("(합계 %s)", fmtEok(flow.Institution))
	if math.Abs(instDiff) >= 1.0 {
		sumStr = fmt.Sprintf("(⚠️ 세부합계 %s vs 기관 %s, 차이 %s)", fmtEok(instSum), fmtEok(flow.Institution), fmtEok(instDiff))
	}

	b.WriteString(fmt.Sprintf("           └ 기관 세부(누적): 금융투자 %s · 보험 %s · 투신 %s · 기타금융 %s · 은행 %s · 연기금 %s · 사모 %s %s\n",
		fmtEok(flow.FinInvest),
		fmtEok(flow.Insurance),
		fmtEok(flow.InvTrust),
		fmtEok(flow.EtcFin),
		fmtEok(flow.Bank),
		fmtEok(flow.Pension),
		fmtEok(flow.PrivEquity),
		sumStr,
	))
}

func renderWindowLine(b *strings.Builder, label string, w Window) {
	if !w.OK {
		b.WriteString(fmt.Sprintf("%-22s  데이터 없음\n", label))
		return
	}

	move1h := "N/A"
	if w.Move1hPct != nil {
		move1h = fmtPct(*w.Move1hPct)
	}
	move2h := "N/A"
	if w.Move2hPct != nil {
		move2h = fmtPct(*w.Move2hPct)
	}

	lastStr := ""
	if !w.LastTS.IsZero() {
		lastStr = fmt.Sprintf("  @%s", w.LastTS.In(kstLocation).Format("2006-01-02 15:04:05 KST"))
	}

	reasonStr := ""
	if w.Reason != "" {
		reasonStr = fmt.Sprintf(" [%s]", w.Reason)
	}

	refStr := ""
	if w.PrevClose > 0 {
		unit := ""
		if w.Symbol == "KRW=X" || strings.Contains(label, "원/달러") {
			unit = "원"
		}
		refStr = fmt.Sprintf(" (기준 %.2f%s)", w.PrevClose, unit)
	}

	changeStr := "N/A"
	if w.ChangeOK {
		changeStr = fmtPct(w.ChangePct)
	}
	b.WriteString(fmt.Sprintf("%-22s  %10.4f  기준대비 %s%s   1h %s  2h %s%s%s\n",
		label, w.Current,
		changeStr,
		refStr,
		move1h, move2h,
		lastStr+" ["+w.Freshness+"]",
		reasonStr,
	))
}

func renderYieldLine(b *strings.Builder, label string, w Window) {
	if !w.OK {
		b.WriteString(fmt.Sprintf("%-20s  데이터 없음\n", label))
		return
	}
	toBP := func(changePct *float64) string {
		if changePct == nil {
			return "N/A"
		}
		denom := 1 + *changePct/100
		if math.Abs(denom) < 1e-9 {
			return "N/A"
		}
		ref := w.Current / denom
		return fmt.Sprintf("%+.1fbp", (w.Current-ref)*100)
	}
	var change *float64
	if w.ChangeOK {
		change = &w.ChangePct
	}
	lastStr := ""
	if !w.LastTS.IsZero() {
		lastStr = fmt.Sprintf("  @%s", w.LastTS.In(kstLocation).Format("2006-01-02 15:04:05 KST"))
	}
	reasonStr := ""
	if w.Reason != "" {
		reasonStr = fmt.Sprintf(" [%s]", w.Reason)
	}
	b.WriteString(fmt.Sprintf("%-20s  %7.3f%%  전일 %s   1h %s  2h %s%s%s\n",
		label, w.Current, toBP(change), toBP(w.Move1hPct), toBP(w.Move2hPct), lastStr+" ["+w.Freshness+"]", reasonStr))
}

// RenderJSON은 Pulse를 간단한 JSON 요약으로 변환합니다 (--json 플래그용).
// 실제 JSON 직렬화는 encoding/json을 통해 호출자가 처리합니다.
func PulseToMap(p *Pulse) map[string]any {
	marketMap := func(m Market) map[string]any {
		var price, change any
		if m.Index.OK {
			price = m.Index.Price
			change = m.Index.ChangePct
		}
		flow := map[string]any{"ok": m.Flow.OK, "source": m.Flow.Source, "last_ts": nullableTime(m.Flow.LastTS), "fetched_at": m.Flow.FetchedAt}
		for key, v := range map[string]float64{"foreign": m.Flow.Foreign, "institution": m.Flow.Institution, "individual": m.Flow.Individual, "etc_corp": m.Flow.EtcCorp, "etc_foreign": m.Flow.EtcForeign, "fin_invest": m.Flow.FinInvest, "insurance": m.Flow.Insurance, "inv_trust": m.Flow.InvTrust, "etc_fin": m.Flow.EtcFin, "bank": m.Flow.Bank, "pension": m.Flow.Pension, "priv_equity": m.Flow.PrivEquity} {
			flow[key] = nil
			if m.Flow.OK && (key != "etc_foreign" || m.Flow.EtcForeignOK) {
				flow[key] = v
			}
		}
		return map[string]any{"price": price, "change_pct": change, "ok": m.Index.OK, "index": m.Index, "window": m.IntradayWin, "move_1h": m.IntradayWin.Move1hPct, "move_2h": m.IntradayWin.Move2hPct, "flow": flow, "flow_delta_prev": m.FlowDeltaPrev, "flow_delta_first": m.FlowDeltaAnchor, "flow_delta_1h": m.FlowDelta1h, "flow_delta_2h": m.FlowDelta2h}
	}
	return map[string]any{"schema_version": 2, "ts": p.Now, "date": p.Date, "business_date": p.BusinessDate, "stored_count": p.StoredCount, "kospi": marketMap(p.KOSPI), "kosdaq": marketMap(p.KOSDAQ), "usdkrw": p.USDKRW, "macro": p.Macro, "market_safety": p.Safety, "program_trade": map[string]any{"kospi": p.KOSPIProgram, "kosdaq": p.KOSDAQProgram, "kospi_delta": p.KOSPIProgramDelta, "kosdaq_delta": p.KOSDAQProgramDelta}, "kospi200_future": p.KOSPI200Future, "kosdaq150_future": p.KOSDAQ150Future, "basis_delta_1h": p.BasisDelta1h, "basis_delta_2h": p.BasisDelta2h, "vkospi": p.VKOSPI, "kospi_contributions": p.Contributions, "assessment": p.Assessment, "analysis": p.Analysis, "errors": p.Errors}
}

func observationLabel(ts, fetched time.Time, fresh string) string {
	observed := "관측 시각 미확인"
	if !ts.IsZero() {
		observed = "관측 " + ts.In(kstLocation).Format("2006-01-02 15:04:05 KST")
	}
	return fmt.Sprintf("%s · 조회 시작 %s [%s]", observed, fetched.In(kstLocation).Format("2006-01-02 15:04:05 KST"), fresh)
}

func fmtPct(v float64) string { return format.Percent(v) }
func fmtPctPtr(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return format.Percent(*v)
}
func fmtEok(v float64) string       { return format.EokArrow(v) }
func fmtAmountEok(v float64) string { return format.AmountEok(v) }
func arrowNeutral(v float64) string { return format.ArrowNeutral(v) }

func positivePrice(v float64) string {
	if v <= 0 || !finite(v) {
		return "N/A"
	}
	return fmt.Sprintf("%.2f", v)
}
func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
