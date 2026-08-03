package premarket

import (
	"fmt"
	"strings"
	"time"
)

const kstLayout = "2006-01-02 15:04 KST"

func Render(r *PremarketReport) string {
	var b strings.Builder
	nowKST := r.Timestamp.In(time.FixedZone("KST", 9*3600))

	b.WriteString(fmt.Sprintf("🌅 개장 전 취약도 보드  %s\n", nowKST.Format(kstLayout)))
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 1. 방향축 D
	dLabel := levelLabel(r.VUL.DScore)
	b.WriteString(fmt.Sprintf("🇺🇸 방향축 D = %d/3  [%s]\n", r.Tier1.DScore, dLabel))

	var memStrs []string
	for _, m := range r.Tier1.SemiMembers {
		if m.IsAvailable {
			memStrs = append(memStrs, fmt.Sprintf("%s %+.2f%%", m.Symbol, m.ChangePct))
		}
	}
	memLine := strings.Join(memStrs, " · ")
	if memLine != "" {
		b.WriteString(fmt.Sprintf("  SEMI_COMPOSITE  %+.2f%%   (%s)\n", r.Tier1.SemiComposite, memLine))
	} else {
		b.WriteString(fmt.Sprintf("  SEMI_COMPOSITE  %+.2f%%\n", r.Tier1.SemiComposite))
	}

	divAlert := ""
	if r.Tier1.HasDivAlert {
		divAlert = "  ⚠ DIVERGENCE_ALERT"
	}
	b.WriteString(fmt.Sprintf("  NQ100선물       %+.2f%%  → DIVERGENCE %+.2f%%p%s\n",
		r.Tier1.NQ100Change, r.Tier1.Divergence, divAlert))

	skhyPremStr := fmt.Sprintf("%+.1f%%", r.Tier1.SKHYPremium)
	if r.Tier1.SKHYClose == 0 {
		skhyPremStr = "N/A"
	}
	ndfGapStr := fmt.Sprintf("%+.1f원", r.Tier1.NDFGap)
	if r.Tier1.NDFClose == 0 {
		ndfGapStr = "N/A"
	}
	b.WriteString(fmt.Sprintf("  SKHY 프리미엄   %s  (전일 %+.1f%%p)  · NDF %.1f (주간종가 대비 %s)\n\n",
		skhyPremStr, r.Tier1.SKHYPremiumChg, r.Tier1.NDFClose, ndfGapStr))

	// 2. 크기축 A
	aLabel := levelLabel(r.VUL.AScore)
	b.WriteString(fmt.Sprintf("⚙️ 크기축 A = %d/3  [%s]\n", r.Tier2.AScore, aLabel))

	vkLine := fmt.Sprintf("%.2f → 기대범위 ±%.1f%%/일", r.Tier2.VKOSPI, r.Tier2.SigmaDaily)
	if r.Tier2.VKOSPIPctile250d > 0 {
		vkLine += fmt.Sprintf(" (250d 백분위 %.0f%%)", r.Tier2.VKOSPIPctile250d)
	}
	if r.Tier2.SpreadVIX != 0 {
		vkLine += fmt.Sprintf("  · VKOSPI-VIX %+.1fp", r.Tier2.SpreadVIX)
	}
	b.WriteString("  VKOSPI " + vkLine + "\n")

	creditTrillion := r.Tier2.CreditLoanBalanceEok / 10000.0
	forcedSellAlert := ""
	if r.Tier2.ForcedSellRatioPct >= 5.0 {
		forcedSellAlert = " ⚠"
	}
	b.WriteString(fmt.Sprintf("  신용융자 %.1f조 (60d pct %.0f%%) · 미수금 대비 반대매매 %.1f%%%s\n",
		creditTrillion, r.Tier2.CreditLoanPctile, r.Tier2.ForcedSellRatioPct, forcedSellAlert))

	levAlert := ""
	if r.Tier2.LevTurnoverRatio >= 0.5 {
		levAlert = " ⚠"
	}
	depositTrillion := r.Tier2.CustomerDepositEok / 10000.0
	b.WriteString(fmt.Sprintf("  레버리지 회전비 %.1f%s · 예탁금 %.1f조 (5일 연속 감소)\n",
		r.Tier2.LevTurnoverRatio, levAlert, depositTrillion))

	marginAlert := ""
	if r.Tier2.HasMarginCascade {
		marginAlert = " ⚠"
	}
	b.WriteString(fmt.Sprintf("  마진콜 근접: 삼성전자 %.1f%%/-15 · SK하이닉스 %.1f%%/-15%s\n\n",
		r.Tier2.ProximitySamsung, r.Tier2.ProximityHynix, marginAlert))

	// 3. 일정축 S
	sLabel := levelLabel(r.VUL.SScore)
	b.WriteString(fmt.Sprintf("📅 일정축 S = %d/3  [%s]\n", r.Tier2.SScore, sLabel))
	if len(r.Tier2.EchoCalendar) > 0 {
		e := r.Tier2.EchoCalendar[0]
		b.WriteString(fmt.Sprintf("  금일 개장: T+2 에코 착지 (원천 %s %+.2f%%, score %.1f)\n",
			e.SourceDate, e.SourceDrop, e.Pressure))
	} else {
		b.WriteString("  금일 개장: T+2 에코 착지 없음\n")
	}
	b.WriteString("  D-1: SK하이닉스 실적 (7/29) · D-3: 레버리지 예탁금 규제 (7/31)\n")
	b.WriteString("  향후: 7/30 개장 에코 후보 + FOMC + 삼성전자 실적 [3중 중첩]\n\n")

	// 4. 취약도 종합 VUL
	gradeStr := r.VUL.OverallGrade
	if r.VUL.Suppressed {
		gradeStr = "SUPPRESSED (신뢰도 미달)"
	}
	if r.VUL.SelfCheckFail {
		gradeStr += " [SELF-CHECK FAIL]"
	}

	b.WriteString(fmt.Sprintf("🧮 취약도 종합: %s  (신뢰도 %.0f%% · 결측 %d/%d)\n",
		gradeStr, r.VUL.ConfidencePct, r.VUL.MissingCount, r.VUL.TotalFields))
	b.WriteString("  → 용도: 포지션 사이징·시나리오 확률 사전 조정. 방향 베팅 신호 아님.\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	return b.String()
}

func levelLabel(score int) string {
	switch {
	case score >= 3:
		return "RED"
	case score == 2:
		return "AMBER"
	default:
		return "GREEN"
	}
}
