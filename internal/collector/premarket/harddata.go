package premarket

import (
	"fmt"
	"strings"
	"time"
)

func collectHardData(in map[string]Observation) []HardDataRow {
	rows := []HardDataRow{}
	for _, d := range []struct{ key, name string }{{"^GSPC", "S&P500"}, {"^IXIC", "나스닥"}, {"^DJI", "다우"}, {"^VIX", "VIX"}, {"^TNX", "미 10년물"}} {
		rows = append(rows, observationRow(d.name, in[d.key]))
	}
	w, b := observationRow("WTI", in["CL=F"]), observationRow("브렌트", in["BZ=F"])
	rows = append(rows, HardDataRow{Item: "WTI/브렌트", Value: "WTI " + w.Value + " / 브렌트 " + b.Value, ChangeDir: "WTI " + w.ChangeDir + " / 브렌트 " + b.ChangeDir, RefTime: "WTI " + w.RefTime + " / 브렌트 " + b.RefTime})
	for _, d := range []struct{ key, name string }{{"DX-Y.NYB", "DXY"}, {"kospi", "코스피"}, {"kosdaq", "코스닥"}, {"foreign_flow", "외국인 수급(코스피)"}, {"KRW=X", "USD/KRW"}, {"vkospi", "VKOSPI"}} {
		rows = append(rows, observationRow(d.name, in[d.key]))
	}
	rows = append(rows, HardDataRow{Item: "코스피200 야간선물", Value: "확인 불가", ChangeDir: "-", RefTime: "실제 야간 세션 자료 미확보"})
	rows = append(rows, observationRow("EWY", in["EWY"]))
	return rows
}
func observationRow(name string, o Observation) HardDataRow {
	r := HardDataRow{Item: name, Value: "확인 불가", ChangeDir: "-", RefTime: o.Reason}
	if !o.available() {
		return r
	}
	r.Value = fmtComma(*o.Value, 2)
	switch o.Unit {
	case "USD":
		r.Value = "$" + r.Value
	case "KRW/USD":
		r.Value += "원"
	case "%":
		r.Value = fmt.Sprintf("%.3f%%", *o.Value)
	case "억원":
		r.Value = fmtComma(*o.Value, 2) + "억 원"
	}
	if name == "외국인 수급(코스피)" {
		switch {
		case *o.Value > 0:
			r.ChangeDir = "순매수"
		case *o.Value < 0:
			r.ChangeDir = "순매도"
		default:
			r.ChangeDir = "순매수·순매도 균형"
		}
	} else if o.Unit == "%" && o.PreviousClose != nil {
		r.ChangeDir = fmt.Sprintf("%+.1fbp", (*o.Value-*o.PreviousClose)*100)
	} else if o.ChangePct != nil {
		r.ChangeDir = fmt.Sprintf("%+.2f%%", *o.ChangePct)
	}
	r.RefTime = observationRef(o)
	return r
}
func observationRef(o Observation) string {
	if !o.available() {
		return "확인 불가"
	}
	ref := formatHyphenDate(o.BusinessDate)
	if o.Session == "DAILY_CLOSE" {
		ref += " 일별 종가"
	} else if o.Session == "DAILY_FLOW" {
		ref += " 일별 수급"
	} else if o.Session == "DAILY_BREADTH" {
		ref += " 전종목 종가 집계"
	} else if o.Session == "DAILY_STATISTIC" {
		ref += " 후행 통계"
	} else if o.Session == "PROVIDER_DAILY" && o.AsOf != nil {
		ref = "제공사 일별값 · 바 시각 " + o.AsOf.In(kst).Format("2006-01-02 15:04 KST") + " (정산가 미검증)"
	} else if o.AsOf != nil {
		ref = o.AsOf.In(kst).Format("2006-01-02 15:04 KST") + " 제공사 관측"
	}
	return strings.TrimSpace(ref) + " · " + o.Source
}
func fmtComma(val float64, decimals int) string {
	rounded := fmt.Sprintf("%.*f", decimals, val)
	parts := strings.SplitN(rounded, ".", 2)
	integer := parts[0]
	sign := ""
	if strings.HasPrefix(integer, "-") {
		sign = "-"
		integer = integer[1:]
	}
	chunks := []string{}
	for len(integer) > 3 {
		chunks = append([]string{integer[len(integer)-3:]}, chunks...)
		integer = integer[:len(integer)-3]
	}
	chunks = append([]string{integer}, chunks...)
	result := sign + strings.Join(chunks, ",")
	if len(parts) > 1 {
		result += "." + parts[1]
	}
	return result
}
func formatHyphenDate(date string) string {
	if len(date) == 8 {
		return date[:4] + "-" + date[4:6] + "-" + date[6:]
	}
	return date
}
func numeric(v *float64, format string) string {
	if v == nil {
		return "N/A"
	}
	return fmt.Sprintf(format, *v)
}
func collectedAt(t time.Time) string { return t.In(kst).Format("2006-01-02 15:04 KST") }
