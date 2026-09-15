package snapshot

import (
	"fmt"
	"strings"
)

func renderCumulative(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 5. 누적 지표와 시총 참고 비중\n")
	c := s.Cumulative
	month := int(s.Timestamp.Month())
	if c == nil {
		b.WriteString("- " + na("cumulative section unavailable") + "\n\n")
		return
	}
	monthly := na(c.MonthlyReason)
	if c.MonthlyForeignNetSellEok != nil {
		monthly = trillionFromEok(*c.MonthlyForeignNetSellEok)
		if c.MonthlyForeignNote != "" {
			monthly += " (" + c.MonthlyForeignNote + ")"
		}
	}
	holding := na(c.ForeignHoldingReason)
	if c.ForeignHoldingChangePP != nil {
		holding = pp(*c.ForeignHoldingChangePP)
	}
	ratio := valuePercent(c.SamsungSKHynixCapRatio, c.CapRatioReason)
	b.WriteString(fmt.Sprintf("- %d월 외국인 누적 매도: %s\n", month, monthly))
	b.WriteString("- 외국인 보유 비중 변화 (1개월): " + holding + "\n")
	b.WriteString("- 삼성전자+SK하이닉스 KIS 마스터 참고 비중: " + ratio + " · " + c.CapRatioStatus + " (마스터 " + c.MasterDate + ", 전일 시총 필드)\n\n")
}

func renderMacro(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 6. 매크로\n")
	m := s.Macro
	if m == nil {
		b.WriteString("- " + na(sectionErr(s, "macro")) + "\n\n")
		return
	}
	for _, item := range []struct{ symbol, label, unit string }{{"KRW=X", "USD/KRW Yahoo indicative quote", " KRW/USD"}, {"CL=F", "WTI 선물", " USD"}, {"^TNX", "미국 10년물 수익률", "%"}} {
		if q, ok := m.Quotes[item.symbol]; ok {
			b.WriteString(fmt.Sprintf("- %s: %s%s · %s\n", item.label, number(q.Price, 2), item.unit, quoteTime(q)))
		} else {
			b.WriteString("- " + item.label + ": N/A\n")
		}
	}
	b.WriteString("\n")
}

func sectionErr(s *Snapshot, section string) string {
	if s != nil && s.Errors != nil {
		if err, ok := s.Errors[section]; ok {
			return errText(err)
		}
	}
	return "section unavailable"
}
