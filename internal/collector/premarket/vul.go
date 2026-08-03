package premarket

func calculateVulnerabilityMatrix(t1 Tier1Direction, t2 Tier2Amplification, t3 Tier3Character) VulnerabilityMatrix {
	vul := VulnerabilityMatrix{
		DScore:      t1.DScore,
		AScore:      t2.AScore,
		SScore:      t2.SScore,
		TotalFields: 23,
	}

	missing := 0
	if len(t1.QualityFlags) > 0 {
		missing += len(t1.QualityFlags)
	}
	if len(t2.QualityFlags) > 0 {
		missing += len(t2.QualityFlags)
	}
	vul.MissingCount = missing

	conf := (1.0 - (float64(missing) / float64(vul.TotalFields))) * 100.0
	if conf < 0 {
		conf = 0
	}
	vul.ConfidencePct = conf

	// Confidence Gating: Suppress grade if missing > 40% (confidence < 60%)
	if conf < 60.0 {
		vul.Suppressed = true
	}

	// Grade Rules:
	// CRITICAL : D=3 AND A>=2 AND S>=2
	// RED      : D>=2 AND (A+S)>=3
	// AMBER    : Any axis >= 2
	// GREEN    : Otherwise
	switch {
	case vul.DScore == 3 && vul.AScore >= 2 && vul.SScore >= 2:
		vul.OverallGrade = "CRITICAL"
	case vul.DScore >= 2 && (vul.AScore+vul.SScore) >= 3:
		vul.OverallGrade = "RED"
	case vul.DScore >= 2 || vul.AScore >= 2 || vul.SScore >= 2:
		vul.OverallGrade = "AMBER"
	default:
		vul.OverallGrade = "GREEN"
	}

	// Self-check for internal contradictions
	if vul.DScore == 0 && t2.SigmaDaily >= 4.5 {
		vul.SelfCheckFail = true
	}

	return vul
}
