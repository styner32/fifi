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
		storeInfo = fmt.Sprintf("당일 적립 %d회 · 직전 %s", p.StoredCount, prevKST.Format("15:04"))
	} else if p.StoredCount > 0 {
		storeInfo = fmt.Sprintf("당일 적립 %d회", p.StoredCount)
	} else {
		storeInfo = "첫 실행 — 기준선 적립 중"
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
	b.WriteString("🚨 시장 안전장치 상태 및 근접도 (공식/추정 결합)\n")
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
	b.WriteString("🧱 KOSPI 시총 상위 10종목 기여도 추정 (전일 종가 가중치 기준)\n")
	if len(p.Contributions) == 0 {
		b.WriteString("  데이터 없음\n")
	} else {
		var top10WeightSum float64
		var top10ImpactSum float64
		for _, c := range p.Contributions {
			top10WeightSum += c.WeightPct
			top10ImpactSum += c.PointImpact
			b.WriteString(fmt.Sprintf("  %-10s %-6s  등락 %+.2f%% · 비중 %.2f%% · %+.2fp\n",
				c.Name, c.Code, c.ChangePct, c.WeightPct, c.PointImpact))
		}
		indexChangePoints := 0.0
		if p.KOSPI.Index.OK && p.KOSPI.Index.PrevClose > 0 {
			indexChangePoints = p.KOSPI.Index.Price - p.KOSPI.Index.PrevClose
		}
		explainedRatio := 0.0
		if math.Abs(indexChangePoints) > 0.0001 {
			explainedRatio = (top10ImpactSum / indexChangePoints) * 100
		}
		b.WriteString(fmt.Sprintf("  └ 상위 10종목 합계: 비중 %.2f%% · 지수 영향합 %+.2fp / 전체 변동 %+.2fp (설명력 %.1f%%)\n",
			top10WeightSum, top10ImpactSum, indexChangePoints, explainedRatio))
	}
	b.WriteString("\n")

	// ── 6. 국내 파생·변동성 ─────────────────────────────────────────────────
	b.WriteString("🌡️ 국내 파생·변동성\n")
	renderDomesticDerivatives(&b, p)
	b.WriteString("\n")

	// ── 7. 환율 ───────────────────────────────────────────────────────────────
	b.WriteString("💱 환율\n")
	renderWindowLine(&b, "  원/달러", p.USDKRW)
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

	// ── 9. 시장반영 분석 ──────────────────────────────────────────────────────
	b.WriteString("🧭 시장반영\n")
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
		b.WriteString(fmt.Sprintf("          시 %.2f / 고 %.2f / 저 %.2f",
			idx.Open, idx.High, idx.Low))
		if idx.Advancers+idx.Decliners > 0 {
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

	etcFrgnStr := ""
	if math.Abs(flow.EtcForeign) > 0.001 {
		etcFrgnStr = fmt.Sprintf(" · 기타외국인 %s", fmtEok(flow.EtcForeign))
	}

	total := flow.Foreign + flow.Institution + flow.Individual + flow.EtcCorp + flow.EtcForeign
	sumStr := "(합계 0억)"
	if math.Abs(total) >= 1.0 {
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
		b.WriteString(fmt.Sprintf("           당일시초대비: 외국인 %s · 기관 %s · 개인 %s · 기타법인 %s%s%s  (경과 %.1f분)\n",
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
		b.WriteString(fmt.Sprintf("           당일시초대비: 차익 %s / 비차익 %s / 합계 %s  (경과 %.1f분)\n",
			fmtEok(anchor.Arbitrage), fmtEok(anchor.NonArbitrage), fmtEok(anchor.Total), anchor.Elapsed))
	}
	if d1h != nil {
		b.WriteString(fmt.Sprintf("           최근1h: 차익 %s / 비차익 %s / 합계 %s\n",
			fmtEok(d1h.Arbitrage), fmtEok(d1h.NonArbitrage), fmtEok(d1h.Total)))
	}
	if cur.AsOf == "close" {
		b.WriteString("           └ 장마감 적용\n")
	} else if len(cur.AsOf) >= 4 {
		b.WriteString(fmt.Sprintf("           └ 집계시점 %s:%s\n", cur.AsOf[:2], cur.AsOf[2:4]))
	}
}

func renderMarketSafety(b *strings.Builder, safety MarketSafety) {
	if len(safety.Devices) == 0 {
		b.WriteString("  장치 데이터 없음\n")
		return
	}
	for _, d := range safety.Devices {
		distStr := "N/A"
		if d.ThresholdDistancePct != nil {
			distStr = fmt.Sprintf("%.2f%%p", *d.ThresholdDistancePct)
		}

		statusDetail := d.EligibilityReason
		if d.State == "TRIGGERED" {
			statusDetail = fmt.Sprintf("실제 발동 중 (발동시각 %s · 해제예정 %s)", d.TriggeredAt, d.ReleasedAt)
		} else if d.State == "RELEASED" {
			statusDetail = fmt.Sprintf("발동 후 해제됨 (발동시각 %s)", d.TriggeredAt)
		} else if d.State == "CONDITION_OBSERVED" {
			statusDetail = fmt.Sprintf("조건 관측 충족 (관측시각 %s · 공식 확인 대기)", d.ConditionObservedAt)
		}

		b.WriteString(fmt.Sprintf("  [%s] %s (임계 %.1f%%): 상태 %s (간격 %s) · %s\n",
			d.Market, d.Device, d.Threshold, d.State, distStr, statusDetail))
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
	if p.KOSPI200Future.OK {
		regime := "콘탱고"
		if p.KOSPI200Future.Basis < 0 {
			regime = "백워데이션"
		}
		b.WriteString(fmt.Sprintf("  KOSPI200 선물 %s  %.2f (%+.2f%%) · 현물 %.2f · 베이시스 %+.2fp (%s)\n",
			p.KOSPI200Future.Code, p.KOSPI200Future.Price, p.KOSPI200Future.ChangePct,
			p.KOSPI200Future.SpotPrice, p.KOSPI200Future.Basis, regime))
		if p.BasisDeltaPrev != nil {
			b.WriteString(fmt.Sprintf("           직전대비 베이시스 변동: %+.2fp\n", p.BasisDeltaPrev.Value))
		}
		if p.BasisDeltaAnchor != nil {
			b.WriteString(fmt.Sprintf("           당일시초대비 베이시스 변동: %+.2fp\n", p.BasisDeltaAnchor.Value))
		}
		if p.BasisDelta1h != nil {
			b.WriteString(fmt.Sprintf("           최근1h 베이시스 변동: %+.2fp\n", p.BasisDelta1h.Value))
		}
	} else {
		b.WriteString("  KOSPI200 베이시스 데이터 없음\n")
	}
	if p.VKOSPI.OK {
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
		b.WriteString(fmt.Sprintf("%-20s  데이터 없음\n", label))
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
		lastStr = fmt.Sprintf("  @%s", w.LastTS.In(kstLocation).Format("15:04"))
	}

	reasonStr := ""
	if w.Freshness == "HOLIDAY" || w.Freshness == "STALE" {
		reasonStr = fmt.Sprintf(" [%s]", w.Reason)
	}

	b.WriteString(fmt.Sprintf("%-20s  %10.4f  전일 %s%s%%   1h %s  2h %s%s%s\n",
		label, w.Current,
		arrowNeutral(w.ChangePct), fmt.Sprintf("%.2f", w.ChangePct),
		move1h, move2h,
		lastStr,
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
	change := w.ChangePct
	lastStr := ""
	if !w.LastTS.IsZero() {
		lastStr = fmt.Sprintf("  @%s", w.LastTS.In(kstLocation).Format("15:04"))
	}
	reasonStr := ""
	if w.Freshness == "HOLIDAY" || w.Freshness == "STALE" {
		reasonStr = fmt.Sprintf(" [%s]", w.Reason)
	}
	b.WriteString(fmt.Sprintf("%-20s  %7.3f%%  전일 %s   1h %s  2h %s%s%s\n",
		label, w.Current, toBP(&change), toBP(w.Move1hPct), toBP(w.Move2hPct), lastStr, reasonStr))
}

// RenderJSON은 Pulse를 간단한 JSON 요약으로 변환합니다 (--json 플래그용).
// 실제 JSON 직렬화는 encoding/json을 통해 호출자가 처리합니다.
func PulseToMap(p *Pulse) map[string]any {
	nowKST := p.Now.In(kstLocation)
	return map[string]any{
		"ts":            nowKST.Format(time.RFC3339),
		"date":          p.Date,
		"business_date": p.BusinessDate,
		"stored_count":  p.StoredCount,
		"kospi": map[string]any{
			"price":      p.KOSPI.Index.Price,
			"change_pct": p.KOSPI.Index.ChangePct,
			"move_1h":    p.KOSPI.IntradayWin.Move1hPct,
			"move_2h":    p.KOSPI.IntradayWin.Move2hPct,
			"flow": map[string]any{
				"foreign":     p.KOSPI.Flow.Foreign,
				"institution": p.KOSPI.Flow.Institution,
				"individual":  p.KOSPI.Flow.Individual,
				"etc_corp":    p.KOSPI.Flow.EtcCorp,
				"etc_foreign": p.KOSPI.Flow.EtcForeign,
				"fin_invest":  p.KOSPI.Flow.FinInvest,
				"insurance":   p.KOSPI.Flow.Insurance,
				"inv_trust":   p.KOSPI.Flow.InvTrust,
				"etc_fin":     p.KOSPI.Flow.EtcFin,
				"bank":        p.KOSPI.Flow.Bank,
				"pension":     p.KOSPI.Flow.Pension,
				"priv_equity": p.KOSPI.Flow.PrivEquity,
			},
		},
		"kosdaq": map[string]any{
			"price":      p.KOSDAQ.Index.Price,
			"change_pct": p.KOSDAQ.Index.ChangePct,
			"move_1h":    p.KOSDAQ.IntradayWin.Move1hPct,
			"move_2h":    p.KOSDAQ.IntradayWin.Move2hPct,
			"flow": map[string]any{
				"foreign":     p.KOSDAQ.Flow.Foreign,
				"institution": p.KOSDAQ.Flow.Institution,
				"individual":  p.KOSDAQ.Flow.Individual,
				"etc_corp":    p.KOSDAQ.Flow.EtcCorp,
				"etc_foreign": p.KOSDAQ.Flow.EtcForeign,
				"fin_invest":  p.KOSDAQ.Flow.FinInvest,
				"insurance":   p.KOSDAQ.Flow.Insurance,
				"inv_trust":   p.KOSDAQ.Flow.InvTrust,
				"etc_fin":     p.KOSDAQ.Flow.EtcFin,
				"bank":        p.KOSDAQ.Flow.Bank,
				"pension":     p.KOSDAQ.Flow.Pension,
				"priv_equity": p.KOSDAQ.Flow.PrivEquity,
			},
		},
		"usdkrw": map[string]any{
			"price":   p.USDKRW.Current,
			"move_1h": p.USDKRW.Move1hPct,
		},
		"market_safety": p.Safety,
		"program_trade": map[string]any{
			"kospi": p.KOSPIProgram, "kosdaq": p.KOSDAQProgram,
			"kospi_delta": p.KOSPIProgramDelta, "kosdaq_delta": p.KOSDAQProgramDelta,
		},
		"kospi200_future":     p.KOSPI200Future,
		"basis_delta_1h":      p.BasisDelta1h,
		"basis_delta_2h":      p.BasisDelta2h,
		"vkospi":              p.VKOSPI,
		"kospi_contributions": p.Contributions,
		"analysis":            p.Analysis,
		"errors":              p.Errors,
	}
}

func fmtPct(v float64) string { return format.Percent(v) }
func fmtPctPtr(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return format.Percent(*v)
}
func fmtEok(v float64) string { return format.EokArrow(v) }
func fmtAmountEok(v float64) string { return format.AmountEok(v) }
func arrowNeutral(v float64) string { return format.ArrowNeutral(v) }

