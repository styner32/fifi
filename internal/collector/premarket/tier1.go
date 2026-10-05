package premarket

import "math"

func collectTier1(in map[string]Observation) Tier1Direction {
	t := Tier1Direction{SKHYClose: in["SKHY"].Value, SKHYRet: in["SKHY"].ChangePct,
		EWYClose: in["EWY"].Value, EWYChange: in["EWY"].ChangePct, VIX: in["^VIX"].Value,
		NQ100Change: in["NQ=F"].ChangePct, Status: "NOT_EVALUATED"}
	defs := []struct {
		symbol, name string
		weight       float64
	}{
		{"SKHY", "SK하이닉스 ADR", .35}, {"^SOX", "필라델피아 반도체지수", .25},
		{"MU", "마이크론", .15}, {"NVDA", "엔비디아", .15}, {"ASML", "ASML", .10}}
	sum, weight := 0.0, 0.0
	session, reference := "", ""
	aligned := true
	for _, d := range defs {
		o := in[d.symbol]
		ok := o.available() && o.ChangePct != nil && o.BusinessDate != "" && o.ReferenceDate != ""
		t.SemiMembers = append(t.SemiMembers, SemiMember{Symbol: d.symbol, Name: d.name, Weight: d.weight, ChangePct: o.ChangePct, IsAvailable: ok, BusinessDate: o.BusinessDate})
		if !ok {
			aligned = false
			continue
		}
		if session == "" {
			session, reference = o.BusinessDate, o.ReferenceDate
		}
		if o.BusinessDate != session || o.ReferenceDate != reference {
			aligned = false
		}
		sum += d.weight * *o.ChangePct
		weight += d.weight
	}
	t.SemiWeightCoveragePct = weight * 100
	if aligned && math.Abs(weight-1) < 1e-9 {
		t.SemiComposite = ptr(sum)
		in["semi_composite"] = Observation{Value: ptr(sum), Source: "US daily closes: 35/25/15/15/10", Unit: "%", BusinessDate: session, ReferenceDate: reference, Session: "DAILY_CLOSE", Status: "VALID", FetchedAt: in["SKHY"].FetchedAt}
	} else {
		in["semi_composite"] = missing("MISSING_OR_MISALIGNED_MEMBERS", in["SKHY"].FetchedAt)
	}
	if t.SKHYRet != nil && math.Abs(*t.SKHYRet) >= 3 {
		t.QualityFlags = append(t.QualityFlags, "SKHY_PRICE_MOVE_ALERT")
	}
	// Continuous futures quotes and KRW=X do not establish a same-contract
	// divergence or an NDF gap. Missing inputs cannot be substituted with zero.
	if t.SemiComposite != nil && in["divergence"].available() && in["ndf_gap"].available() {
		score := 0
		switch {
		case *t.SemiComposite <= -3.5:
			score = 3
		case *t.SemiComposite <= -2:
			score = 2
		case *t.SemiComposite <= -1:
			score = 1
		}
		t.Divergence = in["divergence"].Value
		t.NDFGap = in["ndf_gap"].Value
		if math.Abs(*t.Divergence) >= 2 {
			score++
		}
		if *t.NDFGap >= 7 && score < 1 {
			score = 1
		}
		if score > 3 {
			score = 3
		}
		t.DScore, t.Status = &score, "EVALUATED"
	}
	return t
}
