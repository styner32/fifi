package premarket

import (
	"context"
	"math"
)

func collectTier1(ctx context.Context, deps Deps, spotPrevClose, usdkrw float64) Tier1Direction {
	t1 := Tier1Direction{}
	if deps.Yahoo == nil {
		t1.QualityFlags = append(t1.QualityFlags, "YAHOO_NIL")
		return t1
	}

	symbols := []string{"SKHY", "^SOX", "MU", "SNDK", "WDC", "NVDA", "ASML", "EWY", "^VIX", "NQ=F", "KRW=X"}
	quotes, err := deps.Yahoo.GetQuotes(ctx, symbols)
	if err != nil || len(quotes) == 0 {
		t1.QualityFlags = append(t1.QualityFlags, "YAHOO_QUOTES_FAILED")
	}

	// 1. SKHY ADR
	if q, ok := quotes["SKHY"]; ok && q.Price > 0 {
		t1.SKHYClose = q.Price
		t1.SKHYRet = q.ChangePercent
		if spotPrevClose > 0 && usdkrw > 0 {
			// ADR conversion ratio: 10 ADRs = 1 Korea share
			convertedKRW := q.Price * 10.0 * usdkrw
			t1.SKHYPremium = (convertedKRW/spotPrevClose - 1.0) * 100.0
		}
		if math.Abs(t1.SKHYRet) >= 6.0 {
			t1.QualityFlags = append(t1.QualityFlags, "SKHY_RED_ALERT")
		} else if math.Abs(t1.SKHYRet) >= 3.0 {
			t1.QualityFlags = append(t1.QualityFlags, "SKHY_AMBER_ALERT")
		}
	} else {
		t1.QualityFlags = append(t1.QualityFlags, "SKHY_MISSING")
	}

	// 2. US Semiconductor Composite Index (SEMI_COMPOSITE)
	memberDefs := []struct {
		sym    string
		name   string
		weight float64
	}{
		{"SKHY", "SK하이닉스 ADR", 0.35},
		{"^SOX", "필라델피아 반도체지수", 0.25},
		{"MU", "마이크론/메모리", 0.15},
		{"NVDA", "엔비디아", 0.15},
		{"ASML", "ASML", 0.10},
	}

	var sumWeight float64
	var sumWeightedRet float64

	for _, m := range memberDefs {
		mem := SemiMember{Symbol: m.sym, Name: m.name, Weight: m.weight}
		if q, ok := quotes[m.sym]; ok && q.Price > 0 {
			mem.ChangePct = q.ChangePercent
			mem.IsAvailable = true
			sumWeight += m.weight
			sumWeightedRet += m.weight * q.ChangePercent
		}
		t1.SemiMembers = append(t1.SemiMembers, mem)
	}

	if sumWeight > 0 {
		t1.SemiComposite = sumWeightedRet / sumWeight
	}

	if nq, ok := quotes["NQ=F"]; ok && nq.Price > 0 {
		t1.NQ100Change = nq.ChangePercent
		t1.Divergence = t1.SemiComposite - nq.ChangePercent
		if math.Abs(t1.Divergence) >= 3.5 {
			t1.HasDivAlert = true
			t1.QualityFlags = append(t1.QualityFlags, "DIVERGENCE_ALERT")
		}
	}

	// 3. EWY
	if ewy, ok := quotes["EWY"]; ok && ewy.Price > 0 {
		t1.EWYClose = ewy.Price
		t1.EWYChange = ewy.ChangePercent
		// Volume ratio can be estimated if quote available
		t1.EWYVolumeRatio = 1.0
		if math.Abs(ewy.ChangePercent) >= 2.0 {
			t1.HasEWYFlowEvent = true
			t1.QualityFlags = append(t1.QualityFlags, "EWY_FLOW_EVENT")
		}
	}

	// 4. VIX
	if vix, ok := quotes["^VIX"]; ok && vix.Price > 0 {
		t1.VIX = vix.Price
	}

	// 5. NDF Gap (Indicative from KRW=X quote)
	if krw, ok := quotes["KRW=X"]; ok && krw.Price > 0 {
		t1.NDFClose = krw.Price
		if usdkrw > 0 {
			t1.NDFGap = krw.Price - usdkrw
		}
	}

	// Calculate D Score (0 ~ 3)
	dScore := 0
	switch {
	case t1.SemiComposite <= -3.5:
		dScore = 3
	case t1.SemiComposite <= -2.0:
		dScore = 2
	case t1.SemiComposite <= -1.0:
		dScore = 1
	default:
		dScore = 0
	}

	if math.Abs(t1.Divergence) >= 2.0 {
		dScore++
		if dScore > 3 {
			dScore = 3
		}
	}
	if t1.NDFGap >= 7.0 && dScore < 1 {
		dScore = 1
	}

	t1.DScore = dScore
	return t1
}
