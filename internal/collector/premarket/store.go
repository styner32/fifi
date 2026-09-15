package premarket

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type DailyRecord struct {
	Date           string                 `json:"date"`
	ObservedAt     time.Time              `json:"observed_at"`
	Inputs         map[string]Observation `json:"inputs"`
	DomesticInputs map[string]Observation `json:"domestic_inputs,omitempty"`
	Calendar       *CalendarReport        `json:"calendar,omitempty"`
}
type StoreData struct {
	SchemaVersion int           `json:"schema_version"`
	Records       []DailyRecord `json:"records"`
}
type Store struct {
	mu       sync.RWMutex
	filePath string
	Data     StoreData
	Err      error
}

func NewStore(dir string) *Store {
	if dir == "" {
		dir = ".cache/premarket"
	}
	// Legacy store.json mixed missing zeroes, report dates and placeholder data.
	// Leave that evidence intact; only v2 observations can enter new statistics.
	s := &Store{filePath: filepath.Join(dir, "store.v2.json"), Data: StoreData{SchemaVersion: 2}}
	raw, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		return s
	}
	if err != nil {
		s.Err = err
		return s
	}
	if err = json.Unmarshal(raw, &s.Data); err != nil {
		s.Err = err
		return s
	}
	if s.Data.SchemaVersion != 2 {
		s.Err = fmt.Errorf("unsupported premarket history schema")
	}
	return s
}
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	sort.Slice(s.Data.Records, func(i, j int) bool { return s.Data.Records[i].Date < s.Data.Records[j].Date })
	if len(s.Data.Records) > 400 {
		s.Data.Records = s.Data.Records[len(s.Data.Records)-400:]
	}
	raw, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.filePath)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".premarket-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.filePath)
}
func (s *Store) UpsertRecord(rec DailyRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.Data.Records {
		if r.Date == rec.Date {
			if !rec.ObservedAt.Before(r.ObservedAt) {
				s.Data.Records[i] = rec
			}
			return
		}
	}
	s.Data.Records = append(s.Data.Records, rec)
}
func (s *Store) History(key, beforeSource, beforeReport string, limit int) []float64 {
	if s == nil || s.Err != nil || beforeSource == "" || limit <= 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	type sample struct {
		value    float64
		observed time.Time
	}
	byDate := map[string]sample{}
	for _, r := range s.Data.Records {
		if r.Date >= beforeReport {
			continue
		}
		o := r.Inputs[key]
		if !o.available() || normalizeDate(o.BusinessDate) == "" || o.BusinessDate >= beforeSource {
			continue
		}
		if key == "vkospi" && o.Session != "DAILY_CLOSE" {
			continue
		}
		if previous, ok := byDate[o.BusinessDate]; !ok || r.ObservedAt.After(previous.observed) {
			byDate[o.BusinessDate] = sample{*o.Value, r.ObservedAt}
		}
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	out := []float64{}
	for _, d := range dates {
		out = append(out, byDate[d].value)
		if len(out) == limit {
			break
		}
	}
	return out
}
