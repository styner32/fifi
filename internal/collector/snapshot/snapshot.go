package snapshot

import (
	"context"
)

func Collect(ctx context.Context, deps Deps, opts Options) *Snapshot {
	date, ts := normalizeDate(opts.Date)
	s := &Snapshot{Timestamp: ts, Errors: map[string]error{}}
	if section, err := collectPrice(ctx, deps.DomesticStock, date); err != nil {
		s.Errors["price"] = err
	} else {
		s.Price = section
	}
	if section, err := collectFlow(ctx, deps.DomesticStock, date); err != nil {
		s.Errors["flow"] = err
	} else {
		s.Flow = section
	}
	if section, err := collectGlobal(ctx, deps.Yahoo); err != nil {
		s.Errors["global"] = err
	} else {
		s.Global = section
	}
	if section, err := collectMacro(ctx, deps.Yahoo, opts); err != nil {
		s.Errors["macro"] = err
	} else {
		s.Macro = section
	}
	s.Impact = collectImpact(ctx, deps, date, s.Flow, s.Price, opts)
	s.Cumulative = collectCumulative(ctx, deps.DomesticStock, date, opts)
	// Section 7: 변동성
	indexChange := 0.0
	if s.Price != nil && s.Price.PreviousClose > 0 {
		indexChange = (s.Price.Close - s.Price.PreviousClose) / s.Price.PreviousClose * 100
	}
	s.Volatility = collectVolatility(ctx, deps.DomesticStock, deps.Naver, deps.Yahoo, deps.Facts, indexChange, date, opts)
	// Section 8: 신용잔고 + 반대매매
	if section, err := collectCredit(ctx, deps.DomesticStock, deps.KOFIA, date); err != nil {
		s.Errors["credit"] = err
	} else {
		s.Credit = section
	}
	// Section 9: 매크로 채널/Regime
	s.Regime = collectRegime(ctx, deps.Yahoo, s.Price, s.Volatility, s.Impact, s.Macro, s.Credit)
	// Section 10: 집중도 (KOSPI 마스터 재사용)
	s.Concentration = collectConcentration(ctx, deps.DomesticStock, date)

	// Section 11: 막판 수급 및 Capitulation 분석
	if section, err := collectLateSession(ctx, deps, date, s.Price, opts); err != nil {
		s.Errors["late_session"] = err
	} else {
		s.LateSession = section
	}

	return s
}
