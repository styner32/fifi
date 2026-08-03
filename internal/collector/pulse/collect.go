package pulse

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
)

// Collect는 전체 펄스를 수집하고 적립합니다.
func Collect(ctx context.Context, deps Deps, opts Options) *Pulse {
	now := deps.Clock()
	nowKST := now.In(kstLocation)
	date := nowKST.Format("20060102")
	storeDir := resolveStoreDir(opts)

	pulse := &Pulse{
		Now:          now,
		Date:         date,
		BusinessDate: date,
		Errors:       map[string]string{},
		StoreDir:     storeDir,
	}
	if businessDate, err := resolveBusinessDate(ctx, deps.Stock, date); err != nil {
		pulse.Errors["market_time"] = err.Error()
	} else {
		pulse.BusinessDate = businessDate
	}

	// ── 1. 독립 시세를 제한 병렬 수집 ───────────────────────────────────────
	var kospiFlow, kosdaqFlow FlowSnapshot
	var kospiIdx, kosdaqIdx IndexLevel
	var kospiProgram, kosdaqProgram ProgramTradeSnapshot
	var kospi200Future, kosdaq150Future IndexFutureSnapshot
	var vkospi VolatilitySnapshot
	var wins map[string]Window
	marketErrors := map[string]string{}
	var kospiFlowErr, kosdaqFlowErr, kospiIdxErr, kosdaqIdxErr error
	var kospiProgramErr, kosdaqProgramErr, kospiFutureErr, kosdaqFutureErr, vkospiErr error

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(6)
	g.Go(func() error { kospiFlow, kospiFlowErr = collectFlow(gctx, deps.Stock, "KSP", "0001", now); return nil })
	g.Go(func() error { kosdaqFlow, kosdaqFlowErr = collectFlow(gctx, deps.Stock, "KSQ", "1001", now); return nil })
	g.Go(func() error { kospiIdx, kospiIdxErr = collectIndex(gctx, deps.Stock, "0001", now); return nil })
	g.Go(func() error { kosdaqIdx, kosdaqIdxErr = collectIndex(gctx, deps.Stock, "1001", now); return nil })
	g.Go(func() error { kospiProgram, kospiProgramErr = collectProgramTrade(gctx, deps.Stock, "K"); return nil })
	g.Go(func() error { kosdaqProgram, kosdaqProgramErr = collectProgramTrade(gctx, deps.Stock, "Q"); return nil })
	g.Go(func() error {
		kospi200Future, kospiFutureErr = collectIndexFuture(gctx, deps.Future, pulse.BusinessDate, "KOSPI200", now)
		kosdaq150Future, kosdaqFutureErr = collectIndexFuture(gctx, deps.Future, pulse.BusinessDate, "KOSDAQ150", now)
		return nil
	})
	g.Go(func() error { vkospi, vkospiErr = collectVKOSPI(gctx, deps.Stock, deps.Naver, now); return nil })
	g.Go(func() error { wins = collectMarket(gctx, deps.Yahoo, now, marketErrors); return nil })
	_ = g.Wait()

	for key, item := range map[string]error{
		"kospi_flow": kospiFlowErr, "kosdaq_flow": kosdaqFlowErr,
		"kospi_index": kospiIdxErr, "kosdaq_index": kosdaqIdxErr,
		"kospi_program": kospiProgramErr, "kosdaq_program": kosdaqProgramErr,
		"kospi200_future": kospiFutureErr, "kosdaq150_future": kosdaqFutureErr,
		"vkospi": vkospiErr,
	} {
		if item != nil {
			pulse.Errors[key] = item.Error()
		}
	}
	for key, value := range marketErrors {
		pulse.Errors[key] = value
	}
	if kospi200Future.OK && !kospi200Future.BasisMatch {
		pulse.Errors["kospi200_basis"] = fmt.Sprintf("계산 베이시스 %.2f와 KIS mrkt_basis %.2f 불일치", kospi200Future.Basis, kospi200Future.MarketBasis)
	}

	// ── 2. KOSPI 기여도 (지수 수집 후) ─────────────────────────────────────
	contributions, contributionErr := collectContributions(ctx, deps.Stock, pulse.BusinessDate, kospiIdx)
	if contributionErr != nil {
		pulse.Errors["kospi_contribution"] = contributionErr.Error()
	}
	if wins == nil {
		wins = map[string]Window{}
	}

	usdkrw := wins["KRW=X"]
	kospiWin := wins["^KS11"]
	kosdaqWin := wins["^KQ11"]
	macroWins := []Window{
		wins["NQ=F"],
		wins["ES=F"],
		wins["YM=F"],
		wins["^N225"],
		wins["CL=F"],
		wins["^TNX"],
	}

	// ── 5. 레코드 로드 → 델타 계산 ─────────────────────────────────────────
	records, loadErr := LoadRecords(storeDir, date)
	if loadErr != nil {
		pulse.Errors["store_load"] = loadErr.Error()
	}
	pulse.StoredCount = len(records)
	var prevRec, anchorRec *PulseRecord
	if len(records) > 0 {
		prevRec = &records[len(records)-1]
		anchorRec = &records[0]
		pulse.PrevTS = &prevRec.TS
	}

	// 수급 델타 분할 계산 (Prev, Anchor, 1h, 2h)
	kospiFlowDeltaPrev := computeSingleFlowDelta(prevRec, kospiFlow, kospiIdx.Price, now, "kospi")
	kosdaqFlowDeltaPrev := computeSingleFlowDelta(prevRec, kosdaqFlow, kosdaqIdx.Price, now, "kosdaq")

	kospiFlowDeltaAnchor := computeSingleFlowDelta(anchorRec, kospiFlow, kospiIdx.Price, now, "kospi")
	kosdaqFlowDeltaAnchor := computeSingleFlowDelta(anchorRec, kosdaqFlow, kosdaqIdx.Price, now, "kosdaq")

	kospiFlowDelta1h, kospiFlowDelta2h := computeFlowDeltas(records, now, kospiFlow, kospiIdx.Price, "kospi")
	kosdaqFlowDelta1h, kosdaqFlowDelta2h := computeFlowDeltas(records, now, kosdaqFlow, kosdaqIdx.Price, "kosdaq")

	// 프로그램 매매 델타 분할 계산 (Prev, Anchor, 1h, 2h)
	kospiProgramDeltaPrev := computeProgramDeltaForRef(prevRec, kospiProgram, now, "kospi")
	kosdaqProgramDeltaPrev := computeProgramDeltaForRef(prevRec, kosdaqProgram, now, "kosdaq")

	kospiProgramDeltaAnchor := computeProgramDeltaForRef(anchorRec, kospiProgram, now, "kospi")
	kosdaqProgramDeltaAnchor := computeProgramDeltaForRef(anchorRec, kosdaqProgram, now, "kosdaq")

	var prev1h = LoadNearest(records, now.Add(-time.Hour))
	var prev2h = LoadNearest(records, now.Add(-2*time.Hour))
	kospiProgramDelta1h := computeProgramDeltaForRef(prev1h, kospiProgram, now, "kospi")
	kospiProgramDelta2h := computeProgramDeltaForRef(prev2h, kospiProgram, now, "kospi")
	kosdaqProgramDelta1h := computeProgramDeltaForRef(prev1h, kosdaqProgram, now, "kosdaq")
	kosdaqProgramDelta2h := computeProgramDeltaForRef(prev2h, kosdaqProgram, now, "kosdaq")

	// 베이시스 델타 분할 계산
	computeBasisDeltaForRef := func(prev *PulseRecord, cur IndexFutureSnapshot, now time.Time) *BasisDelta {
		if prev == nil || !prev.KOSPI200Future.OK || !cur.OK {
			return nil
		}
		return &BasisDelta{
			RefTS: prev.TS,
			Elapsed: now.Sub(prev.TS).Minutes(),
			Value: cur.Basis - prev.KOSPI200Future.Basis,
		}
	}
	basisDeltaPrev := computeBasisDeltaForRef(prevRec, kospi200Future, now)
	basisDeltaAnchor := computeBasisDeltaForRef(anchorRec, kospi200Future, now)
	basisDelta1h := computeBasisDeltaForRef(prev1h, kospi200Future, now)
	basisDelta2h := computeBasisDeltaForRef(prev2h, kospi200Future, now)


	// Let's refine the residual calculations:
	if kospiFlow.OK {
		kospiTotalSum := kospiFlow.Individual + kospiFlow.Foreign + kospiFlow.Institution + kospiFlow.EtcCorp + kospiFlow.EtcForeign
		kospiResTotal := math.Abs(kospiTotalSum)
		kospiInstSum := kospiFlow.FinInvest + kospiFlow.InvTrust + kospiFlow.Pension + kospiFlow.PrivEquity + kospiFlow.Insurance + kospiFlow.Bank + kospiFlow.EtcFin
		kospiResInst := math.Abs(kospiFlow.Institution - kospiInstSum)

		if kospiResTotal > 10.0 {
			pulse.Errors["kospi_flow_residual"] = fmt.Sprintf("KOSPI 수급 합계 불일치 잔차 %.2f억원 (>10억원)", kospiTotalSum)
		}
		if kospiResInst > 10.0 {
			pulse.Errors["kospi_inst_residual"] = fmt.Sprintf("KOSPI 기관 세부합계 불일치 잔차 %.2f억원 (>10억원)", kospiFlow.Institution - kospiInstSum)
		}
	}
	if kosdaqFlow.OK {
		kosdaqTotalSum := kosdaqFlow.Individual + kosdaqFlow.Foreign + kosdaqFlow.Institution + kosdaqFlow.EtcCorp + kosdaqFlow.EtcForeign
		kosdaqResTotal := math.Abs(kosdaqTotalSum)
		kosdaqInstSum := kosdaqFlow.FinInvest + kosdaqFlow.InvTrust + kosdaqFlow.Pension + kosdaqFlow.PrivEquity + kosdaqFlow.Insurance + kosdaqFlow.Bank + kosdaqFlow.EtcFin
		kosdaqResInst := math.Abs(kosdaqFlow.Institution - kosdaqInstSum)

		if kosdaqResTotal > 10.0 {
			pulse.Errors["kosdaq_flow_residual"] = fmt.Sprintf("KOSDAQ 수급 합계 불일치 잔차 %.2f억원 (>10억원)", kosdaqTotalSum)
		}
		if kosdaqResInst > 10.0 {
			pulse.Errors["kosdaq_inst_residual"] = fmt.Sprintf("KOSDAQ 기관 세부합계 불일치 잔차 %.2f억원 (>10억원)", kosdaqFlow.Institution - kosdaqInstSum)
		}
	}

	// ── 6. 시장 구조체 조립 ─────────────────────────────────────────────────
	pulse.KOSPI = Market{
		Name:            "KOSPI",
		Index:           kospiIdx,
		IntradayWin:     kospiWin,
		Flow:            kospiFlow,
		FlowDeltaPrev:   kospiFlowDeltaPrev,
		FlowDeltaAnchor: kospiFlowDeltaAnchor,
		FlowDelta1h:     kospiFlowDelta1h,
		FlowDelta2h:     kospiFlowDelta2h,
	}
	pulse.KOSDAQ = Market{
		Name:            "KOSDAQ",
		Index:           kosdaqIdx,
		IntradayWin:     kosdaqWin,
		Flow:            kosdaqFlow,
		FlowDeltaPrev:   kosdaqFlowDeltaPrev,
		FlowDeltaAnchor: kosdaqFlowDeltaAnchor,
		FlowDelta1h:     kosdaqFlowDelta1h,
		FlowDelta2h:     kosdaqFlowDelta2h,
	}
	pulse.KOSPIProgram = kospiProgram
	pulse.KOSDAQProgram = kosdaqProgram
	pulse.KOSPIProgramDelta = kospiProgramDelta1h // Legacy compat
	pulse.KOSPIProgramDeltaPrev = kospiProgramDeltaPrev
	pulse.KOSPIProgramDeltaAnchor = kospiProgramDeltaAnchor
	pulse.KOSPIProgramDelta1h = kospiProgramDelta1h
	pulse.KOSPIProgramDelta2h = kospiProgramDelta2h

	pulse.KOSDAQProgramDelta = kosdaqProgramDelta1h // Legacy compat
	pulse.KOSDAQProgramDeltaPrev = kosdaqProgramDeltaPrev
	pulse.KOSDAQProgramDeltaAnchor = kosdaqProgramDeltaAnchor
	pulse.KOSDAQProgramDelta1h = kosdaqProgramDelta1h
	pulse.KOSDAQProgramDelta2h = kosdaqProgramDelta2h

	pulse.KOSPI200Future = kospi200Future
	pulse.KOSDAQ150Future = kosdaq150Future
	pulse.BasisDelta1h = basisDelta1h // Legacy compat
	pulse.BasisDelta2h = basisDelta2h // Legacy compat
	pulse.BasisDeltaPrev = basisDeltaPrev
	pulse.BasisDeltaAnchor = basisDeltaAnchor
	pulse.VKOSPI = vkospi
	pulse.Safety = buildMarketSafety(now, date, kospiIdx, kosdaqIdx, kospi200Future, kosdaq150Future, records)
	pulse.Contributions = contributions
	pulse.USDKRW = usdkrw
	pulse.Macro = macroWins

	// ── 7. 분석 ────────────────────────────────────────────────────────────
	pulse.Analysis = Analyze(pulse)

	// ── 8. 적립 (--no-save가 아닌 경우) ────────────────────────────────────
	if !opts.NoSave {
		rec := PulseRecord{
			TS:              now,
			BusinessDate:    pulse.BusinessDate,
			KOSPIIdx:        kospiIdx.Price,
			KOSDAQIdx:       kosdaqIdx.Price,
			KOSPIFlow:       kospiFlow,
			KOSDAQFlow:      kosdaqFlow,
			KOSPIProgram:    kospiProgram,
			KOSDAQProgram:   kosdaqProgram,
			KOSPI200Future:  kospi200Future,
			KOSDAQ150Future: kosdaq150Future,
			VKOSPI:          vkospi,
			USDKRW:          usdkrw.Current,
		}

		if appendErr := AppendRecord(storeDir, date, rec); appendErr != nil {
			pulse.Errors["store_append"] = appendErr.Error()
		} else {
			pulse.StoredCount++
			pulse.Saved = true
		}
	}

	return pulse
}

// computeFlowDeltas는 1h / 2h 수급 델타를 계산합니다.
func computeFlowDeltas(records []PulseRecord, now time.Time, cur FlowSnapshot, curIdx float64, market string) (*FlowDelta, *FlowDelta) {
	if !cur.OK {
		return nil, nil
	}

	var delta1h, delta2h *FlowDelta
	for _, h := range []int{1, 2} {
		target := now.Add(-time.Duration(h) * time.Hour)
		prev := LoadNearest(records, target)
		if prev == nil {
			continue
		}
		elapsed := now.Sub(prev.TS).Minutes()

		var prevFlow FlowSnapshot
		var prevIdx float64
		if market == "kospi" {
			prevFlow = prev.KOSPIFlow
			prevIdx = prev.KOSPIIdx
		} else {
			prevFlow = prev.KOSDAQFlow
			prevIdx = prev.KOSDAQIdx
		}

		d := &FlowDelta{
			RefTS:       prev.TS,
			Elapsed:     elapsed,
			Foreign:     cur.Foreign - prevFlow.Foreign,
			Institution: cur.Institution - prevFlow.Institution,
			Individual:  cur.Individual - prevFlow.Individual,
			IndexDelta:  curIdx - prevIdx,
		}
		if h == 1 {
			delta1h = d
		} else {
			delta2h = d
		}
	}
	return delta1h, delta2h
}

// FlowAcceleration은 1h 델타가 직전 1h 대비 가속/둔화를 판단합니다.
func FlowAcceleration(delta1h, delta2h *FlowDelta, field func(*FlowDelta) float64) string {
	if delta1h == nil || delta2h == nil {
		return ""
	}
	currentMinutes := delta1h.Elapsed
	previousMinutes := delta2h.Elapsed - delta1h.Elapsed
	if currentMinutes <= 0 || previousMinutes <= 0 {
		return ""
	}
	currentRate := hourlyRate(field(delta1h), currentMinutes)
	previousRate := hourlyRate(field(delta2h)-field(delta1h), previousMinutes)
	if sign(currentRate) != 0 && sign(previousRate) != 0 && sign(currentRate) != sign(previousRate) {
		if currentRate > 0 {
			return "매수전환"
		}
		return "매도전환"
	}
	if absDelta(currentRate) > absDelta(previousRate) {
		return "가속"
	}
	if absDelta(currentRate) < absDelta(previousRate) {
		return "둔화"
	}
	return "유지"
}

func absDelta(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// resolveStoreDir는 옵션/환경변수/기본값 순으로 적립 디렉터리를 반환합니다.
func resolveStoreDir(opts Options) string {
	if opts.StoreDir != "" {
		return opts.StoreDir
	}
	if v := os.Getenv("PULSE_OUTPUT_DIR"); v != "" {
		return v
	}
	return ".cache/pulse"
}

// StorePath는 적립 JSONL 경로를 반환합니다.
func StorePath(opts Options, date string) string {
	return pulseFilePath(resolveStoreDir(opts), date)
}

// MDPath는 MD 저장 경로를 반환합니다.
func MDPath(opts Options, date string) string {
	return pulseMDPath(resolveStoreDir(opts), date)
}

// SaveMD는 렌더 결과를 MD 파일로 저장합니다.
func SaveMD(opts Options, date, content string) error {
	dir := resolveStoreDir(opts)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(pulseMDPath(dir, date), []byte(content), 0o644)
}

func computeSingleFlowDelta(prevRec *PulseRecord, cur FlowSnapshot, curIdx float64, now time.Time, market string) *FlowDelta {
	if prevRec == nil || !cur.OK {
		return nil
	}
	var prevFlow FlowSnapshot
	var prevIdx float64
	if market == "kospi" {
		prevFlow = prevRec.KOSPIFlow
		prevIdx = prevRec.KOSPIIdx
	} else {
		prevFlow = prevRec.KOSDAQFlow
		prevIdx = prevRec.KOSDAQIdx
	}
	if !prevFlow.OK {
		return nil
	}
	return &FlowDelta{
		RefTS:       prevRec.TS,
		Elapsed:     now.Sub(prevRec.TS).Minutes(),
		Foreign:     cur.Foreign - prevFlow.Foreign,
		Institution: cur.Institution - prevFlow.Institution,
		Individual:  cur.Individual - prevFlow.Individual,
		IndexDelta:  curIdx - prevIdx,
	}
}

func computeProgramDeltaForRef(prevRec *PulseRecord, cur ProgramTradeSnapshot, now time.Time, market string) *ProgramTradeDelta {
	if prevRec == nil || !cur.OK {
		return nil
	}
	var previous ProgramTradeSnapshot
	if market == "kospi" {
		previous = prevRec.KOSPIProgram
	} else {
		previous = prevRec.KOSDAQProgram
	}
	if !previous.OK {
		return nil
	}
	return &ProgramTradeDelta{
		RefTS:        prevRec.TS,
		Elapsed:      now.Sub(prevRec.TS).Minutes(),
		Arbitrage:    cur.Arbitrage - previous.Arbitrage,
		NonArbitrage: cur.NonArbitrage - previous.NonArbitrage,
		Total:        cur.Total - previous.Total,
	}
}
