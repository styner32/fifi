package snapshot

import (
	"fmt"
	"strings"
)


func renderCumulative(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 5. 누적 지표 [수동 입력]\n")
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
	b.WriteString("- 삼성전자+SK하이닉스 시총 비중: " + ratio + "\n\n")
}

func renderMacro(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 6. 매크로\n")
	m := s.Macro
	if m == nil {
		b.WriteString("- " + na(sectionErr(s, "macro")) + "\n")
		return
	}
	if q, ok := m.Quotes["KRW=X"]; ok {
		line := number(q.Price, 2)
		if m.USDKRWMonthStart != nil && m.USDKRWMonthStartPct != nil {
			line += fmt.Sprintf(" (%d월초 %s 대비 %s)", int(s.Timestamp.Month()), number(*m.USDKRWMonthStart, 0), percent(*m.USDKRWMonthStartPct))
		}
		b.WriteString("- USD/KRW: " + line + "\n")
	} else {
		b.WriteString("- USD/KRW: " + na(m.Reason) + "\n")
	}
	if q, ok := m.Quotes["CL=F"]; ok {
		b.WriteString("- WTI: $" + number(q.Price, 2) + "\n")
	} else {
		b.WriteString("- WTI: " + na(m.Reason) + "\n")
	}
	if q, ok := m.Quotes["^TNX"]; ok {
		// ^TNX regularMarketPrice is already in percent units (e.g. 4.52 = 4.52%)
		b.WriteString("- 미국 10년물: " + number(q.Price, 2) + "%\n")
	} else {
		b.WriteString("- 미국 10년물: " + na(m.Reason) + "\n")
	}
}

func sectionErr(s *Snapshot, section string) string {
	if s != nil && s.Errors != nil {
		if err, ok := s.Errors[section]; ok {
			return errText(err)
		}
	}
	return "section unavailable"
}
