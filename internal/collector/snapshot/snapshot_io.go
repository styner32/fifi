package snapshot

import (
	"encoding/json"
	"fmt"
	"github.com/fifi/internal/fileio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SnapshotJSON은 JSON 직렬화용 구조체입니다.
// Snapshot 내부의 에러 맵은 직렬화가 안 되므로 주요 수치만 포함합니다.
type SnapshotJSON struct {
	SchemaVersion int                   `json:"schema_version"`
	SessionStatus string                `json:"session_status"`
	CompletedAt   time.Time             `json:"completed_at"`
	Impact        *ImpactSection        `json:"impact"`
	Cumulative    *CumulativeSection    `json:"cumulative"`
	Date          string                `json:"date"`
	Timestamp     time.Time             `json:"timestamp"`
	Price         *PriceSection         `json:"price,omitempty"`
	Flow          *FlowSection          `json:"flow,omitempty"`
	Volatility    *VolatilitySection    `json:"volatility,omitempty"`
	Credit        *CreditSection        `json:"credit,omitempty"`
	Regime        *RegimeSection        `json:"regime,omitempty"`
	Concentration *ConcentrationSection `json:"concentration,omitempty"`
	Global        *GlobalSectionJSON    `json:"global,omitempty"`
	Macro         *MacroSectionJSON     `json:"macro,omitempty"`
	LateSession   *LateSessionSection   `json:"late_session,omitempty"`
	Errors        map[string]string     `json:"errors,omitempty"`
}

// GlobalSectionJSON은 GlobalSection에서 비교에 필요한 필드만 추출합니다.
type GlobalSectionJSON struct {
	Quotes map[string]QuoteSummary `json:"quotes,omitempty"`
}

// MacroSectionJSON은 MacroSection에서 비교에 필요한 필드만 추출합니다.
type MacroSectionJSON struct {
	Quotes map[string]QuoteSummary `json:"quotes,omitempty"`
}

// QuoteSummary는 종목 시세 요약입니다.
type QuoteSummary struct {
	PreviousClose  float64 `json:"previous_close"`
	MarketTimeUnix int64   `json:"market_time_unix"`
	Currency       string  `json:"currency"`
	Source         string  `json:"source"`
	Price          float64 `json:"price"`
	ChangePercent  float64 `json:"change_percent"`
}

// ToJSON은 Snapshot을 JSON 직렬화용 구조체로 변환합니다.
func (s *Snapshot) ToJSON() *SnapshotJSON {
	date := s.BusinessDate
	if date == "" {
		date = s.Timestamp.Format("20060102")
	}
	j := &SnapshotJSON{
		SchemaVersion: 2, SessionStatus: s.SessionStatus, CompletedAt: s.CompletedAt, Impact: s.Impact, Cumulative: s.Cumulative,
		Timestamp:     s.Timestamp,
		Date:          date,
		Price:         s.Price,
		Flow:          s.Flow,
		Volatility:    s.Volatility,
		Credit:        s.Credit,
		Regime:        s.Regime,
		Concentration: s.Concentration,
		LateSession:   s.LateSession,
	}
	if len(s.Errors) > 0 {
		j.Errors = make(map[string]string, len(s.Errors))
		for k, v := range s.Errors {
			if v != nil {
				j.Errors[k] = v.Error()
			}
		}
	}
	if s.Global != nil && len(s.Global.Quotes) > 0 {
		g := &GlobalSectionJSON{Quotes: map[string]QuoteSummary{}}
		for k, q := range s.Global.Quotes {
			g.Quotes[k] = QuoteSummary{Price: q.Price, ChangePercent: q.ChangePercent, PreviousClose: q.PreviousClose, MarketTimeUnix: q.MarketTimeUnix, Currency: q.Currency, Source: "Yahoo"}
		}
		j.Global = g
	}
	if s.Macro != nil && len(s.Macro.Quotes) > 0 {
		m := &MacroSectionJSON{Quotes: map[string]QuoteSummary{}}
		for k, q := range s.Macro.Quotes {
			m.Quotes[k] = QuoteSummary{Price: q.Price, ChangePercent: q.ChangePercent, PreviousClose: q.PreviousClose, MarketTimeUnix: q.MarketTimeUnix, Currency: q.Currency, Source: "Yahoo"}
		}
		j.Macro = m
	}
	return j
}

// SaveJSON은 스냅샷을 날짜별 JSON 파일로 저장합니다.
// 경로: <dir>/market_snapshot.<YYYYMMDD>.json
func SaveJSON(s *Snapshot, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	date := s.ToJSON().Date
	if !validDate(date) {
		return "", fmt.Errorf("invalid snapshot date")
	}
	filePath := filepath.Join(dir, fmt.Sprintf("market_snapshot.%s.json", date))
	// Keep the previous dated artifact before replacing the latest pointer.
	if raw, e := os.ReadFile(filePath); e == nil {
		archive := filepath.Join(dir, fmt.Sprintf("market_snapshot.%s.previous.%d.json", date, time.Now().UnixNano()))
		if e = os.WriteFile(archive, raw, 0600); e != nil {
			return "", e
		}
	}
	if err := fileio.WriteJSONAtomic(filePath, s.ToJSON()); err != nil {
		return "", err
	}

	return filePath, nil
}

var snapshotFileRE = regexp.MustCompile(`^market_snapshot\.(\d{8})\.json$`)

// LoadPreviousSnapshot은 dir에서 targetDate보다 이전인 가장 최근 JSON을 불러옵니다.
// 찾지 못하면 nil, nil을 반환합니다.
func LoadPreviousSnapshot(dir string, targetDate string) (*SnapshotJSON, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// 날짜 문자열 수집 (targetDate보다 이전만)
	normalized := strings.ReplaceAll(targetDate, "-", "")
	var dates []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := snapshotFileRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		d := m[1]
		if d < normalized {
			dates = append(dates, d)
		}
	}
	if len(dates) == 0 {
		return nil, nil
	}
	sort.Strings(dates)
	latest := dates[len(dates)-1]

	filePath := filepath.Join(dir, fmt.Sprintf("market_snapshot.%s.json", latest))
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}

	var prev SnapshotJSON
	if err := json.Unmarshal(raw, &prev); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", filePath, err)
	}
	if prev.Date != latest {
		return nil, fmt.Errorf("snapshot filename/content date mismatch")
	}
	return &prev, nil
}

// LoadSnapshotForDate loads the snapshot JSON for the exact specified date if it exists.
func LoadSnapshotForDate(dir string, date string) (*SnapshotJSON, error) {
	normalized := strings.ReplaceAll(date, "-", "")
	if !validDate(normalized) {
		return nil, fmt.Errorf("invalid snapshot date")
	}
	filePath := filepath.Join(dir, fmt.Sprintf("market_snapshot.%s.json", normalized))
	raw, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s SnapshotJSON
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if s.Date != normalized {
		return nil, fmt.Errorf("snapshot filename/content date mismatch")
	}
	return &s, nil
}

// Preserve missingness in machine-readable output as well as Markdown.
func maskedJSON(value any, fields ...string) ([]byte, error) {
	raw, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	var obj map[string]json.RawMessage
	if e = json.Unmarshal(raw, &obj); e != nil {
		return nil, e
	}
	for _, f := range fields {
		obj[f] = json.RawMessage("null")
	}
	return json.Marshal(obj)
}
func (p PriceSection) MarshalJSON() ([]byte, error) {
	type plain PriceSection
	return maskedJSON(plain(p), p.MissingFields...)
}
func (c ConcentrationSection) MarshalJSON() ([]byte, error) {
	type plain ConcentrationSection
	var fs []string
	if c.Status != StatusEstimated {
		fs = []string{"top_2_percent", "top_5_percent", "top_10_percent", "hhi", "denominator_eok"}
	}
	return maskedJSON(plain(c), fs...)
}
func (r RegimeSection) MarshalJSON() ([]byte, error) {
	type plain RegimeSection
	return maskedJSON(plain(r), "kospi_nasdaq_corr", "kospi_nikkei_corr", "global_risk_aversion_idx")
}
func (v VolatilitySection) MarshalJSON() ([]byte, error) {
	type plain VolatilitySection
	fs := []string{"decoupling_flag"}
	if v.VKOSPI <= 0 {
		fs = append(fs, "vkospi")
	}
	if !v.VKOSPIChangeOK {
		fs = append(fs, "vkospi_change")
	}
	if len(v.AverageDates) != 5 {
		fs = append(fs, "vkospi_5day_avg")
	}
	if v.VIX <= 0 {
		fs = append(fs, "vix")
	}
	if !v.VIXChangeOK {
		fs = append(fs, "vix_change")
	}
	if v.ObservedAt.IsZero() {
		fs = append(fs, "observed_at")
	}
	if v.VIXObservedAt.IsZero() {
		fs = append(fs, "vix_observed_at")
	}
	return maskedJSON(plain(v), fs...)
}
func (q QuoteSummary) MarshalJSON() ([]byte, error) {
	type plain QuoteSummary
	fs := []string{}
	if !finite(q.PreviousClose) || q.PreviousClose <= 0 {
		fs = append(fs, "previous_close", "change_percent")
	} else {
		q.ChangePercent = (q.Price/q.PreviousClose - 1) * 100
	}
	if q.MarketTimeUnix <= 0 {
		fs = append(fs, "market_time_unix")
	}
	return maskedJSON(plain(q), fs...)
}
func (c CreditSection) MarshalJSON() ([]byte, error) {
	type plain CreditSection
	fs := append([]string(nil), c.MissingFields...)
	fs = append(fs, "margin_receivable_prev_eok", "margin_receivable_delta_eok", "forced_sell_amount_prev_eok", "forced_sell_delta_eok")
	if c.KofiaDate == "" {
		fs = append(fs, "margin_receivable_eok", "forced_sell_amount_eok", "forced_sell_ratio_pct", "raw_forced_sell_mln", "raw_margin_receivable_mln")
	}
	return maskedJSON(plain(c), fs...)
}
func (s LateSessionSection) MarshalJSON() ([]byte, error) {
	type plain LateSessionSection
	fs := []string{"is_expiring_contract", "kospi200_standard_futures_expiry", "kospi200_monthly_options_expiry"}
	if !s.PatternEvaluated {
		fs = append(fs, "pattern_detected")
	}
	if !s.BasisOK {
		fs = append(fs, "basis_point", "basis_rate", "futures_price", "spot_price")
	}
	if !s.Futures1530OK {
		fs = append(fs, "futures_price_1530", "basis_point_1530")
	}
	if !s.ProgramOK {
		fs = append(fs, "kospi_net_arbitrage_total", "kospi_net_non_arbitrage_total", "kospi_program_total_net", "original_snapshot_arbitrage_eok", "original_snapshot_non_arbitrage_eok", "original_snapshot_total_eok")
	}
	if !s.InstitutionOK {
		fs = append(fs, "kospi_net_non_arbitrage_organ")
	}
	if !s.ForeignOK {
		fs = append(fs, "kospi_net_non_arbitrage_foreign")
	}
	if s.NaverFollowupTotalEok == nil {
		fs = append(fs, "cross_source_difference_eok")
	}
	if s.FuturesObservedAt.IsZero() {
		fs = append(fs, "futures_observed_at")
	}
	if s.SpotObservedAt.IsZero() {
		fs = append(fs, "spot_observed_at")
	}
	return maskedJSON(plain(s), fs...)
}

func (s ImpactSection) MarshalJSON() ([]byte, error) {
	type plain ImpactSection
	fs := []string{}
	if s.FuturesObservedAt.IsZero() {
		fs = append(fs, "futures_observed_at")
	}
	if s.TotalKospiMarketCapEok <= 0 {
		fs = append(fs, "total_kospi_market_cap_eok")
	}
	return maskedJSON(plain(s), fs...)
}
