package premarket

import (
	"fmt"
	"strings"
)

func renderDomestic(b *strings.Builder, r *PremarketReport) {
	b.WriteString("\n🇰🇷 국내 전일 수급 (억원, 최근 3·5거래일은 전일 포함)\n\n| 시장·투자자 | 전일 | 3거래일 누적 | 5거래일 누적 | 기준·출처 |\n| :--- | ---: | ---: | ---: | :--- |\n")
	for _, m := range []struct{ key, name string }{{"kospi", "코스피"}, {"kosdaq", "코스닥"}} {
		for _, iv := range []struct{ key, name string }{{"foreign", "외국인"}, {"institution", "기관"}, {"individual", "개인"}} {
			prefix := m.key + "_" + iv.key
			fmt.Fprintf(b, "| %s %s | %s | %s | %s | %s |\n", m.name, iv.name, domesticValue(r.DomesticInputs[prefix+"_1d"]), domesticValue(r.DomesticInputs[prefix+"_3d"]), domesticValue(r.DomesticInputs[prefix+"_5d"]), observationRef(r.DomesticInputs[prefix+"_1d"]))
		}
	}
	b.WriteString("\n전일 시장 내부 상태\n\n| 시장 | 상승 / 하락 / 보합 | 상승 비율 | 거래대금(억원) | 이전 20일 평균 대비 | 기준·출처 |\n| :--- | :--- | ---: | ---: | ---: | :--- |\n")
	for _, m := range []struct{ key, name string }{{"kospi", "코스피"}, {"kosdaq", "코스닥"}} {
		in := r.DomesticInputs
		fmt.Fprintf(b, "| %s | %s / %s / %s | %s | %s | %s | 수: %s / 대금: %s |\n", m.name, domesticValue(in[m.key+"_advancers"]), domesticValue(in[m.key+"_decliners"]), domesticValue(in[m.key+"_unchanged"]), domesticValue(in[m.key+"_advancing_pct"]), domesticValue(in[m.key+"_turnover"]), domesticValue(in[m.key+"_turnover_ratio20"]), observationRef(in[m.key+"_advancers"]), observationRef(in[m.key+"_turnover"]))
	}
	b.WriteString("  종목 수: 공공데이터 시장별 주식 전체(우선주·거래정지 포함), 지수 구성종목 수와 다름. 등락 0은 보합.\n  상승 비율 = 상승 / (상승+하락+보합). 공공데이터 미게시 시 확인 불가.\n  거래대금 평균은 전일을 제외한 이전 20거래일, 누적 수급은 거래일별 누락 없이 확보한 경우에만 계산.\n")
	b.WriteString("\n삼성전자·SK하이닉스 (KRX 전일 완료 세션)\n\n| 종목 | 종가 | 등락률 | 거래대금(억원) | 외국인 순매수(억원) | 기관 순매수(억원) | 기준·출처 |\n| :--- | ---: | ---: | ---: | ---: | ---: | :--- |\n")
	for _, s := range []struct{ code, name string }{{"005930", "삼성전자"}, {"000660", "SK하이닉스"}} {
		in := r.DomesticInputs
		price := in[s.code+"_close"]
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s |\n", s.name, domesticValue(price), numeric(price.ChangePct, "%+.2f%%"), domesticValue(in[s.code+"_turnover"]), domesticValue(in[s.code+"_foreign"]), domesticValue(in[s.code+"_institution"]), observationRef(price))
	}
}
func domesticValue(o Observation) string {
	if !o.available() {
		return "확인 불가"
	}
	switch o.Unit {
	case "종목":
		return fmtComma(*o.Value, 0)
	case "KRW/share":
		return fmtComma(*o.Value, 0) + "원"
	case "%":
		return fmt.Sprintf("%.1f%%", *o.Value)
	case "배":
		return fmt.Sprintf("%.2f배", *o.Value)
	default:
		return fmtComma(*o.Value, 2)
	}
}
func renderCalendar(b *strings.Builder, r CalendarReport) {
	fmt.Fprintf(b, "\n공식 일정 (%s ~ %s 미만, 향후 7일) · 수집 상태 %s\n", r.WindowStart, r.WindowEnd, r.Status)
	b.WriteString("| 일정 | 시점 | 상태 | 출처 |\n| :--- | :--- | :--- | :--- |\n")
	for _, e := range r.Events {
		when := e.SourceDate
		if e.SourceEndDate != "" && e.SourceEndDate != e.SourceDate {
			when += " ~ " + e.SourceEndDate
		}
		when += " 미국 현지 날짜 · 시각 미확인"
		if e.ScheduledAt != nil {
			when = e.ScheduledAt.In(kst).Format("01-02 15:04 KST")
		}
		state := "예정"
		if e.State == "SCHEDULED_TIME_PASSED_RELEASE_UNVERIFIED" {
			state = "예정시각 경과 · 실제 발표 미확인"
		}
		if e.ScheduledAt == nil {
			state = "회의 날짜 확인"
		}
		if e.Confirmation == "TENTATIVE" {
			state = "잠정 일정"
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", calendarCell(e.Title), when, state, e.SourceURL)
	}
	if len(r.Events) == 0 {
		b.WriteString("  조회된 출처·기간에서 표시할 일정 없음. 전체 시장의 이벤트 부재를 뜻하지 않음.\n")
	}
	for _, s := range r.Sources {
		fmt.Fprintf(b, "  %s: %s", s.Name, s.Status)
		if s.Reason != "" {
			fmt.Fprintf(b, " (%s)", s.Reason)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "  미연결: %s\n  일정 원문 수집 시각은 보고서 실행 시각. 사후 재실행은 당시 공지 내용의 복원을 보장하지 않음.\n", strings.Join(r.Uncovered, " / "))
}
func calendarCell(s string) string {
	return strings.NewReplacer("|", "/", "\n", " ", "\r", " ").Replace(s)
}
