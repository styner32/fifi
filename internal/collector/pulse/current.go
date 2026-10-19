package pulse

import "strings"

// SetCurrentOnly preserves current source observations and daily changes while
// removing comparisons between observations. It does not write a history row.
func (p *Pulse) SetCurrentOnly() {
	p.CurrentOnly = true
	p.StoredCount, p.PrevTS, p.Saved = 0, nil, false
	for _, market := range []*Market{&p.KOSPI, &p.KOSDAQ} {
		market.FlowDeltaPrev, market.FlowDeltaAnchor = nil, nil
		market.FlowDelta1h, market.FlowDelta2h = nil, nil
		clearWindowComparisons(&market.IntradayWin)
	}
	p.KOSPIProgramDelta, p.KOSDAQProgramDelta = nil, nil
	p.KOSPIProgramDeltaPrev, p.KOSPIProgramDeltaAnchor = nil, nil
	p.KOSPIProgramDelta1h, p.KOSPIProgramDelta2h = nil, nil
	p.KOSDAQProgramDeltaPrev, p.KOSDAQProgramDeltaAnchor = nil, nil
	p.KOSDAQProgramDelta1h, p.KOSDAQProgramDelta2h = nil, nil
	p.BasisDeltaPrev, p.BasisDeltaAnchor = nil, nil
	p.BasisDelta1h, p.BasisDelta2h = nil, nil
	clearWindowComparisons(&p.USDKRW)
	for i := range p.Macro {
		clearWindowComparisons(&p.Macro[i])
	}
	p.Analysis = Analyze(p)
}

func clearWindowComparisons(window *Window) {
	window.Move1hPct, window.Move2hPct = nil, nil
	window.Ref1hTS, window.Ref2hTS = nil, nil
	if strings.HasPrefix(window.Reason, "NO_VALID_WINDOW_ANCHOR") {
		window.Reason = ""
	}
}
