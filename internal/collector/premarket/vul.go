package premarket

import "sort"

func calculateVulnerabilityMatrix(t1 Tier1Direction, t2 Tier2Amplification, in map[string]Observation) VulnerabilityMatrix {
	v := VulnerabilityMatrix{DScore: t1.DScore, AScore: t2.AScore, SScore: t2.SScore, TotalFields: len(in), OverallGrade: "COMPOSITE_NOT_CIRCULABLE", Suppressed: true}
	for key, o := range in {
		if !o.available() {
			v.MissingInputs = append(v.MissingInputs, key)
		}
	}
	sort.Strings(v.MissingInputs)
	v.MissingCount = len(v.MissingInputs)
	if v.TotalFields > 0 {
		v.CoveragePct = float64(v.TotalFields-v.MissingCount) / float64(v.TotalFields) * 100
	}
	if t1.DScore == nil || t2.AScore == nil || t2.SScore == nil {
		return v
	}
	d, a, s := *t1.DScore, *t2.AScore, *t2.SScore
	v.Suppressed = false
	switch {
	case d == 3 && a >= 2 && s >= 2:
		v.OverallGrade = "CRITICAL"
	case d >= 2 && a+s >= 3:
		v.OverallGrade = "RED"
	case d >= 2 || a >= 2 || s >= 2:
		v.OverallGrade = "AMBER"
	default:
		v.OverallGrade = "GREEN"
	}
	return v
}
