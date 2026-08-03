package premarket

import (
	"context"
)

func collectTier3(ctx context.Context, foreignNetSellEok, usdkrwChgPct float64) Tier3Character {
	t3 := Tier3Character{
		CDS5Y: 35.0, // Default 5Y CDS proxy
	}

	// FX-Stock Divergence Classifier
	if foreignNetSellEok <= -10000.0 && usdkrwChgPct <= 0.3 {
		t3.DivergencePhase = "국내 청산 성격 (잠정)"
		t3.DivergenceCaption = "매도대금 환전 T+2 이연 가능 — 향후 2영업일 환율로 재검증"
	} else if foreignNetSellEok <= -10000.0 && usdkrwChgPct >= 0.7 {
		t3.DivergencePhase = "자본 이탈 성격"
		t3.DivergenceCaption = "원화 약세 동반 자본 유출 신호"
		t3.HasCapitalFlight = true
	} else {
		t3.DivergencePhase = "혼조/판정 보류"
		t3.DivergenceCaption = "수급 및 환율 변동 폭 미달"
	}

	return t3
}
