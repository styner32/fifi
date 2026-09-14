package premarket

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/parse"
)

func collectHardData(ctx context.Context, deps Deps, date string, now time.Time) []HardDataRow {
	var rows []HardDataRow

	// 1. Fetch Yahoo quotes
	var quotes map[string]yahoo.Quote
	if deps.Yahoo != nil {
		symbols := []string{
			"^GSPC",
			"^IXIC",
			"^DJI",
			"^VIX",
			"^TNX",
			"CL=F",
			"BZ=F",
			"DX-Y.NYB",
			"DX=F",
			"^KS11",
			"^KQ11",
			"KRW=X",
			"EWY",
		}
		q, _ := deps.Yahoo.GetQuotes(ctx, symbols)
		quotes = q
	}

	if quotes == nil {
		quotes = make(map[string]yahoo.Quote)
	}

	// 1. S&P500
	rows = append(rows, formatIndexRow("S&P500", quotes["^GSPC"]))

	// 2. 나스닥
	rows = append(rows, formatIndexRow("나스닥", quotes["^IXIC"]))

	// 3. 다우
	rows = append(rows, formatIndexRow("다우", quotes["^DJI"]))

	// 4. VIX
	rows = append(rows, formatIndexRow("VIX", quotes["^VIX"]))

	// 5. 미 10년물
	rows = append(rows, formatTreasuryRow(quotes["^TNX"]))

	// 6. WTI/브렌트
	rows = append(rows, formatOilRow(quotes["CL=F"], quotes["BZ=F"]))

	// 7. DXY
	dxyQ, hasDXY := quotes["DX-Y.NYB"]
	if !hasDXY || dxyQ.Price == 0 {
		dxyQ = quotes["DX=F"]
	}
	rows = append(rows, formatIndexRow("DXY", dxyQ))

	// 8. 코스피
	rows = append(rows, formatKOSPIRow(ctx, deps, quotes["^KS11"], date))

	// 9. 코스닥
	rows = append(rows, formatKOSDAQRow(ctx, deps, quotes["^KQ11"], date))

	// 10. 외국인 수급(코스피)
	rows = append(rows, formatForeignFlowRow(ctx, deps, date))

	// 11. USD/KRW
	rows = append(rows, formatUSDKRWRow(quotes["KRW=X"]))

	// 12. VKOSPI
	rows = append(rows, formatVKOSPIRow(ctx, deps, date))

	// 13. 코스피200 야간선물
	rows = append(rows, formatNightFuturesRow(now))

	// 14. EWY
	rows = append(rows, formatEWYRow(quotes["EWY"]))

	return rows
}

func formatRefDate(marketTimeUnix int64, defaultSuffix string) string {
	if marketTimeUnix > 0 {
		t := time.Unix(marketTimeUnix, 0).In(time.FixedZone("KST", 9*3600))
		return t.Format("2006-01-02") + " " + defaultSuffix
	}
	return defaultSuffix
}

func formatIndexRow(name string, q yahoo.Quote) HardDataRow {
	if q.Price == 0 {
		return HardDataRow{Item: name, Value: "확인 불가", ChangeDir: "-", RefTime: "-"}
	}
	diff := q.Price - q.PreviousClose
	arrow := "▲"
	if diff < 0 {
		arrow = "▼"
	} else if diff == 0 {
		arrow = ""
	}
	refDate := formatRefDate(q.MarketTimeUnix, "종가")

	changeStr := fmt.Sprintf("%+.2f%%", q.ChangePercent)
	if arrow != "" {
		changeStr += fmt.Sprintf(" (%s%.2fp)", arrow, math.Abs(diff))
	}

	return HardDataRow{
		Item:      name,
		Value:     fmtComma(q.Price, 2),
		ChangeDir: changeStr,
		RefTime:   refDate,
	}
}

func formatTreasuryRow(q yahoo.Quote) HardDataRow {
	if q.Price == 0 {
		return HardDataRow{Item: "미 10년물", Value: "확인 불가", ChangeDir: "-", RefTime: "-"}
	}
	bpDiff := (q.Price - q.PreviousClose) * 100
	dir := "상승"
	if bpDiff < 0 {
		dir = "하락"
	} else if bpDiff == 0 {
		dir = "보합"
	}
	refDate := formatRefDate(q.MarketTimeUnix, "종가")

	return HardDataRow{
		Item:      "미 10년물",
		Value:     fmt.Sprintf("%.3f%%", q.Price),
		ChangeDir: fmt.Sprintf("%+.1f bp (%s)", bpDiff, dir),
		RefTime:   refDate,
	}
}

func formatOilRow(wti, brent yahoo.Quote) HardDataRow {
	if wti.Price == 0 && brent.Price == 0 {
		return HardDataRow{Item: "WTI/브렌트", Value: "확인 불가", ChangeDir: "-", RefTime: "-"}
	}
	var valParts, chgParts []string
	var refDate string

	if wti.Price > 0 {
		valParts = append(valParts, fmt.Sprintf("WTI $%.2f", wti.Price))
		chgParts = append(chgParts, fmt.Sprintf("WTI %+.2f%%", wti.ChangePercent))
		refDate = formatRefDate(wti.MarketTimeUnix, "종가")
	}
	if brent.Price > 0 {
		valParts = append(valParts, fmt.Sprintf("브렌트 $%.2f", brent.Price))
		chgParts = append(chgParts, fmt.Sprintf("브렌트 %+.2f%%", brent.ChangePercent))
		if refDate == "" {
			refDate = formatRefDate(brent.MarketTimeUnix, "종가")
		}
	}

	return HardDataRow{
		Item:      "WTI/브렌트",
		Value:     strings.Join(valParts, " / "),
		ChangeDir: strings.Join(chgParts, " / "),
		RefTime:   refDate,
	}
}

func formatKOSPIRow(ctx context.Context, deps Deps, yq yahoo.Quote, date string) HardDataRow {
	if deps.Stock != nil {
		if resp, err := deps.Stock.InquireIndexPrice(ctx, "0001"); err == nil && resp != nil {
			if r := resp.FirstRow("output"); r != nil {
				prpr, ok := parse.Num(r, "bstp_nmix_prpr")
				if ok && prpr > 0 {
					vrss, _ := parse.Num(r, "bstp_nmix_prdy_vrss")
					ctrt, _ := parse.Num(r, "bstp_nmix_prdy_ctrt")
					arrow := "▲"
					if vrss < 0 {
						arrow = "▼"
					} else if vrss == 0 {
						arrow = ""
					}
					changeStr := fmt.Sprintf("%+.2f%%", ctrt)
					if arrow != "" {
						changeStr += fmt.Sprintf(" (%s%.2fp)", arrow, math.Abs(vrss))
					}
					refDate := formatHyphenDate(date) + " 종가"
					return HardDataRow{
						Item:      "코스피",
						Value:     fmtComma(prpr, 2),
						ChangeDir: changeStr,
						RefTime:   refDate,
					}
				}
			}
		}
	}
	if yq.Price > 0 {
		return formatIndexRow("코스피", yq)
	}
	return HardDataRow{Item: "코스피", Value: "확인 불가", ChangeDir: "-", RefTime: "-"}
}

func formatKOSDAQRow(ctx context.Context, deps Deps, yq yahoo.Quote, date string) HardDataRow {
	if deps.Stock != nil {
		if resp, err := deps.Stock.InquireIndexPrice(ctx, "1001"); err == nil && resp != nil {
			if r := resp.FirstRow("output"); r != nil {
				prpr, ok := parse.Num(r, "bstp_nmix_prpr")
				if ok && prpr > 0 {
					vrss, _ := parse.Num(r, "bstp_nmix_prdy_vrss")
					ctrt, _ := parse.Num(r, "bstp_nmix_prdy_ctrt")
					arrow := "▲"
					if vrss < 0 {
						arrow = "▼"
					} else if vrss == 0 {
						arrow = ""
					}
					changeStr := fmt.Sprintf("%+.2f%%", ctrt)
					if arrow != "" {
						changeStr += fmt.Sprintf(" (%s%.2fp)", arrow, math.Abs(vrss))
					}
					refDate := formatHyphenDate(date) + " 종가"
					return HardDataRow{
						Item:      "코스닥",
						Value:     fmtComma(prpr, 2),
						ChangeDir: changeStr,
						RefTime:   refDate,
					}
				}
			}
		}
	}
	if yq.Price > 0 {
		return formatIndexRow("코스닥", yq)
	}
	return HardDataRow{Item: "코스닥", Value: "확인 불가", ChangeDir: "-", RefTime: "-"}
}

func formatForeignFlowRow(ctx context.Context, deps Deps, date string) HardDataRow {
	if deps.Stock != nil {
		resp, err := deps.Stock.InquireInvestorDailyByMarket(ctx, date)
		if err == nil && resp != nil {
			if row := resp.FirstRow("output"); row != nil {
				if frgn, ok := parse.Num(row, "frgn_ntby_tr_pbmn"); ok {
					frgnEok := frgn / 100.0 // 백만원 -> 억원
					dir := "순매수"
					if frgnEok < 0 {
						dir = "순매도"
					}
					refDate := formatHyphenDate(date) + " 장마감"
					return HardDataRow{
						Item:      "외국인 수급(코스피)",
						Value:     fmt.Sprintf("%s억 원", fmtComma(frgnEok, 0)),
						ChangeDir: dir,
						RefTime:   refDate,
					}
				}
			}
		}
	}
	return HardDataRow{Item: "외국인 수급(코스피)", Value: "확인 불가", ChangeDir: "-", RefTime: formatHyphenDate(date) + " 장마감"}
}

func formatUSDKRWRow(q yahoo.Quote) HardDataRow {
	if q.Price == 0 {
		return HardDataRow{Item: "USD/KRW", Value: "확인 불가", ChangeDir: "-", RefTime: "-"}
	}
	diff := q.Price - q.PreviousClose
	arrow := "▲"
	if diff < 0 {
		arrow = "▼"
	}
	refDate := formatRefDate(q.MarketTimeUnix, "뉴욕 마감")

	changeStr := fmt.Sprintf("%+.2f%%", q.ChangePercent)
	if diff != 0 {
		changeStr += fmt.Sprintf(" (%s%.2f원)", arrow, math.Abs(diff))
	}

	return HardDataRow{
		Item:      "USD/KRW",
		Value:     fmt.Sprintf("%.2f원", q.Price),
		ChangeDir: changeStr,
		RefTime:   refDate,
	}
}

func formatVKOSPIRow(ctx context.Context, deps Deps, date string) HardDataRow {
	if deps.Stock != nil {
		if vkCode, err := deps.Stock.ResolveVKOSPICode(ctx, nil); err == nil {
			if resp, err := deps.Stock.InquireVKOSPIPrice(ctx, vkCode); err == nil && resp != nil {
				if row := resp.FirstRow("output"); row != nil {
					if prpr, ok := parse.Num(row, "bstp_nmix_prpr"); ok && prpr > 0 {
						vrss, _ := parse.Num(row, "bstp_nmix_prdy_vrss")
						ctrt, _ := parse.Num(row, "bstp_nmix_prdy_ctrt")
						changeStr := fmt.Sprintf("%+.2f%%", ctrt)
						if vrss == 0 {
							changeStr = "0.00% (보합)"
						}
						return HardDataRow{
							Item:      "VKOSPI",
							Value:     fmt.Sprintf("%.2f", prpr),
							ChangeDir: changeStr,
							RefTime:   formatHyphenDate(date) + " 종가",
						}
					}
				}
			}
		}
	}
	if deps.Naver != nil {
		if nq, err := deps.Naver.GetIndexQuote(ctx, "VKOSPI"); err == nil && nq != nil && nq.Price > 0 {
			return HardDataRow{
				Item:      "VKOSPI",
				Value:     fmt.Sprintf("%.2f", nq.Price),
				ChangeDir: fmt.Sprintf("%+.2f%%", nq.ChangePercent),
				RefTime:   formatHyphenDate(date) + " 종가",
			}
		}
	}
	return HardDataRow{Item: "VKOSPI", Value: "확인 불가", ChangeDir: "-", RefTime: "-"}
}

func formatNightFuturesRow(now time.Time) HardDataRow {
	nowKST := now.In(time.FixedZone("KST", 9*3600))
	refTime := nowKST.Format("2006-01-02") + " 06:00 야간마감"
	return HardDataRow{
		Item:      "코스피200 야간선물",
		Value:     "확인 불가",
		ChangeDir: "-",
		RefTime:   refTime,
	}
}

func formatEWYRow(q yahoo.Quote) HardDataRow {
	if q.Price == 0 {
		return HardDataRow{Item: "EWY", Value: "확인 불가", ChangeDir: "-", RefTime: "-"}
	}
	diff := q.Price - q.PreviousClose
	arrow := "▲"
	if diff < 0 {
		arrow = "▼"
	}
	refDate := formatRefDate(q.MarketTimeUnix, "종가")

	changeStr := fmt.Sprintf("%+.2f%%", q.ChangePercent)
	if diff != 0 {
		changeStr += fmt.Sprintf(" (%s%.2f$)", arrow, math.Abs(diff))
	}

	return HardDataRow{
		Item:      "EWY",
		Value:     fmt.Sprintf("$%.2f", q.Price),
		ChangeDir: changeStr,
		RefTime:   refDate,
	}
}

func fmtComma(val float64, decimals int) string {
	sign := ""
	if val < 0 {
		sign = "-"
		val = -val
	}
	intPart := int64(val)
	intStr := fmt.Sprintf("%d", intPart)

	// insert commas into integer part
	var parts []string
	for len(intStr) > 3 {
		parts = append([]string{intStr[len(intStr)-3:]}, parts...)
		intStr = intStr[:len(intStr)-3]
	}
	if len(intStr) > 0 {
		parts = append([]string{intStr}, parts...)
	}
	joined := strings.Join(parts, ",")

	if decimals > 0 {
		fracFmt := fmt.Sprintf("%%0.%df", decimals)
		fullStr := fmt.Sprintf(fracFmt, val)
		dotIdx := strings.Index(fullStr, ".")
		if dotIdx != -1 {
			return sign + joined + fullStr[dotIdx:]
		}
	}
	return sign + joined
}

func formatHyphenDate(yyyymmdd string) string {
	if len(yyyymmdd) == 8 {
		return yyyymmdd[0:4] + "-" + yyyymmdd[4:6] + "-" + yyyymmdd[6:8]
	}
	return yyyymmdd
}
