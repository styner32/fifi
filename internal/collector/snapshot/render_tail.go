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
		b.WriteString("- " + na(sectionErr(s, "macro")) + "\n\n")
		return
	}
	if q, ok := m.Quotes["KRW=X"]; ok {
		b.WriteString(fmt.Sprintf("- USD/KRW Yahoo `KRW=X` indicative quote:\n  - 값: %s\n  - 관측 시각: TIMESTAMP_MISSING\n  - 상태: `INDICATIVE_QUOTE / TIMESTAMP_MISSING`\n", number(q.Price, 2)))
	} else {
		b.WriteString("- USD/KRW Yahoo `KRW=X` indicative quote: " + na(m.Reason) + "\n")
	}
	if q, ok := m.Quotes["CL=F"]; ok {
		b.WriteString(fmt.Sprintf("- WTI:\n  - 값: $%s\n  - 관측 시각: TIMESTAMP_MISSING\n", number(q.Price, 2)))
	} else {
		b.WriteString("- WTI: " + na(m.Reason) + "\n")
	}
	if q, ok := m.Quotes["^TNX"]; ok {
		b.WriteString(fmt.Sprintf("- 미국 10년물:\n  - 값: %s%%\n  - 관측 시각: TIMESTAMP_MISSING\n  - 상태: `TIMESTAMP_MISSING`\n", number(q.Price, 2)))
	} else {
		b.WriteString("- 미국 10년물: " + na(m.Reason) + "\n")
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
