package snapshot

import (
	"encoding/json"
	"fmt"
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
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"change_percent"`
}

// ToJSON은 Snapshot을 JSON 직렬화용 구조체로 변환합니다.
func (s *Snapshot) ToJSON() *SnapshotJSON {
	j := &SnapshotJSON{
		Timestamp:     s.Timestamp,
		Date:          s.Timestamp.Format("20060102"),
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
			g.Quotes[k] = QuoteSummary{Price: q.Price, ChangePercent: q.ChangePercent}
		}
		j.Global = g
	}
	if s.Macro != nil && len(s.Macro.Quotes) > 0 {
		m := &MacroSectionJSON{Quotes: map[string]QuoteSummary{}}
		for k, q := range s.Macro.Quotes {
			m.Quotes[k] = QuoteSummary{Price: q.Price, ChangePercent: q.ChangePercent}
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
	date := s.Timestamp.Format("20060102")
	fileName := fmt.Sprintf("market_snapshot.%s.json", date)
	filePath := filepath.Join(dir, fileName)

	data, err := json.MarshalIndent(s.ToJSON(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
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
	return &prev, nil
}

// LoadSnapshotForDate loads the snapshot JSON for the exact specified date if it exists.
func LoadSnapshotForDate(dir string, date string) (*SnapshotJSON, error) {
	normalized := strings.ReplaceAll(date, "-", "")
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
	return &s, nil
}

