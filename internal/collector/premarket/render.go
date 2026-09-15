package premarket

import (
	"fmt"
	"sort"
	"strings"
)

func Render(r *PremarketReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🌅 개장 전 시장 브리핑  %s\n", collectedAt(r.Timestamp))
	fmt.Fprintf(&b, "대상일 %s · 국내 직전 완료 거래일 %s\n\n", formatHyphenDate(r.Date), dateOrMissing(r.DomesticCloseDate))
	if r.CutoffAt != nil {
		fmt.Fprintf(&b, "자료 선택 상한 %s · 이후 관측은 장전 값으로 사용하지 않음\n\n", collectedAt(*r.CutoffAt))
	}
	b.WriteString("📊 하드 데이터 표 (완료 거래일·제공사 관측 구분)\n\n| 항목 | 값 | 등락/방향 | 기준 시점 |\n| :--- | :--- | :--- | :--- |\n")
	for _, row := range r.HardData {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", row.Item, row.Value, row.ChangeDir, row.RefTime)
	}
	renderDomestic(&b, r)
	fmt.Fprintf(&b, "\n🇺🇸 방향축 D = %s\n", scoreLabel(r.Tier1.DScore))
	fmt.Fprintf(&b, "  SEMI_COMPOSITE %s · 구성 가중치 확보 %.0f%%\n", numeric(r.Tier1.SemiComposite, "%+.2f%%"), r.Tier1.SemiWeightCoveragePct)
	members := []string{}
	for _, m := range r.Tier1.SemiMembers {
		members = append(members, fmt.Sprintf("%s %.0f%%: %s", m.Symbol, m.Weight*100, numeric(m.ChangePct, "%+.2f%%")))
	}
	fmt.Fprintf(&b, "  %s\n  결측 종목의 가중치는 재분배하지 않음 · 동일 거래일·비교일 확인 후 합성\n", strings.Join(members, " · "))
	fmt.Fprintf(&b, "  NQ 제공사 기준 등락 %s · %s\n  DIVERGENCE N/A (계약·비교 기준 미검증)\n", numeric(r.Tier1.NQ100Change, "%+.2f%%"), observationRef(r.Inputs["NQ=F"]))
	b.WriteString("  SKHY 프리미엄 N/A (환산비율·대응 주가 미검증) · NDF 갭 N/A (별도 자료 미확보)\n")
	fmt.Fprintf(&b, "\n⚙️ 크기축 A = %s\n", scoreLabel(r.Tier2.AScore))
	fmt.Fprintf(&b, "  VKOSPI %s → 일간 변동성 환산 %s · %s\n", numeric(r.Tier2.VKOSPI, "%.2f"), numeric(r.Tier2.SigmaDaily, "±%.2f%%"), observationRef(r.Inputs["vkospi"]))
	fmt.Fprintf(&b, "  VKOSPI 250거래일 백분위 %s (이력 %d/250)\n", numeric(r.Tier2.VKOSPIPctile250d, "%.1f%%"), r.Tier2.VKOSPISampleCount)
	for _, d := range []struct{ key, label string }{{"credit", "신용융자"}, {"deposit", "예탁금"}, {"margin", "미수금"}, {"forced", "반대매매 금액"}, {"forced_ratio", "반대매매 비중"}} {
		o := r.Inputs[d.key]
		value := "N/A"
		if o.available() {
			value = fmtComma(*o.Value, 2) + " " + o.Unit
		}
		fmt.Fprintf(&b, "  %s %s · %s\n", d.label, value, observationRef(o))
	}
	fmt.Fprintf(&b, "  신용융자 60거래일 백분위 %s (이력 %d/60)\n", numeric(r.Tier2.CreditLoanPctile, "%.1f%%"), r.Tier2.CreditSampleCount)
	b.WriteString("  레버리지 회전비·예탁금 5일 추세 N/A (미구현)\n  마진콜 근접도 N/A (담보·대출 기준 자료 미확보)\n")
	fmt.Fprintf(&b, "\n📅 일정축 S = %s\n", scoreLabel(r.Tier2.SScore))
	renderCalendar(&b, r.Calendar)
	b.WriteString("  일정 목록은 점수화하지 않음 · S 미평가\n  T+2 에코 확인 불가 (실제 낙폭·결제일 검증 미완료)\n")
	fmt.Fprintf(&b, "\n🧮 취약도 종합: %s\n  평가 입력 확보율 %.1f%% · 미확보 %d/%d · 예측 신뢰도 NOT_EVALUATED\n", r.VUL.OverallGrade, r.VUL.CoveragePct, r.VUL.MissingCount, r.VUL.TotalFields)
	b.WriteString("  D: 반도체·동일 계약 선물·NDF 필요 / A: 변동성·신용·회전비·예탁금 추세 필요 / S: 검증된 일정 필요\n")
	if len(r.Errors) > 0 {
		keys := []string{}
		for key := range r.Errors {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteString("\n수집 오류:\n")
		for _, key := range keys {
			fmt.Fprintf(&b, "  %s: %s\n", key, r.Errors[key])
		}
	}
	return b.String()
}
func scoreLabel(v *int) string {
	if v == nil {
		return "N/A [NOT_EVALUATED]"
	}
	return fmt.Sprintf("%d/3 [%s]", *v, levelLabel(*v))
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
func dateOrMissing(date string) string {
	if date == "" {
		return "확인 불가"
	}
	return formatHyphenDate(date)
}
