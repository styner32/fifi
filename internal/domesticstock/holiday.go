package domesticstock

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fifi/internal/fileio"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/market/calendar"
)

const holidayPath = "/uapi/domestic-stock/v1/quotations/chk-holiday"

type holidayCache struct {
	SchemaVersion int              `json:"schema_version"`
	QueryDate     string           `json:"query_date"`
	CheckedAt     time.Time        `json:"checked_at"`
	Rows          []map[string]any `json:"rows,omitempty"`
	Error         string           `json:"error,omitempty"`
}

// TradingDay uses a retrieval-day cache shared by all target dates. The official
// sample requests at most one daily call; failures and interrupted reservations
// are cached too, so repeated report calls cannot hammer this endpoint.
func (s *Service) TradingDay(ctx context.Context, date string) (calendar.TradingDay, error) {
	dir := strings.TrimSpace(os.Getenv("KIS_HOLIDAY_CACHE_DIR"))
	if dir == "" {
		dir = ".cache"
	}
	return s.tradingDay(ctx, date, time.Now().In(kst.Location), dir)
}

func (s *Service) tradingDay(ctx context.Context, date string, now time.Time, dir string) (calendar.TradingDay, error) {
	if t, err := time.Parse("20060102", date); err != nil || t.Format("20060102") != date {
		return calendar.TradingDay{}, fmt.Errorf("invalid holiday date")
	}
	queryDate := now.In(kst.Location).Format("20060102")
	path := filepath.Join(dir, "kis_holidays."+queryDate+".json")
	if raw, err := os.ReadFile(path); err == nil {
		var cache holidayCache
		if json.Unmarshal(raw, &cache) != nil || cache.SchemaVersion != 1 || cache.QueryDate != queryDate || cache.CheckedAt.IsZero() || cache.CheckedAt.After(now) || cache.CheckedAt.In(kst.Location).Format("20060102") != queryDate {
			return calendar.TradingDay{}, fmt.Errorf("invalid holiday cache; preserved without retry")
		}
		return holidayFromCache(cache, date)
	} else if !os.IsNotExist(err) {
		return calendar.TradingDay{}, fmt.Errorf("read holiday cache: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return calendar.TradingDay{}, err
	}
	// Exclusive reservation also bounds calls across collector/report processes.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return calendar.TradingDay{}, fmt.Errorf("holiday daily query already reserved or cache unavailable")
	}
	cache := holidayCache{SchemaVersion: 1, QueryDate: queryDate, CheckedAt: now, Error: "DAILY_QUERY_IN_PROGRESS_OR_INTERRUPTED"}
	writeErr := json.NewEncoder(f).Encode(cache)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return calendar.TradingDay{}, fmt.Errorf("reserve daily holiday query failed")
	}
	resp, requestErr := s.client.Get(ctx, holidayPath, "CTCA0903R", "", map[string]string{"BASS_DT": queryDate, "CTX_AREA_FK": "", "CTX_AREA_NK": ""})
	cache.Error = ""
	if requestErr != nil {
		cache.Error = "KIS holiday request failed"
	} else if resp == nil || !resp.IsOK() {
		cache.Error = "KIS holiday business response failed"
	} else {
		cache.Rows = resp.Rows("output")
		if len(cache.Rows) == 0 {
			cache.Error = "KIS holiday response has no rows"
		}
	}
	if err := fileio.WriteJSONAtomic(path, cache); err != nil {
		return calendar.TradingDay{}, fmt.Errorf("save holiday result: %w", err)
	}
	return holidayFromCache(cache, date)
}

func holidayFromCache(cache holidayCache, date string) (calendar.TradingDay, error) {
	if cache.Error != "" {
		return calendar.TradingDay{}, fmt.Errorf("%s", cache.Error)
	}
	var found *calendar.TradingDay
	for _, row := range cache.Rows {
		if row["bass_dt"] != date {
			continue
		}
		flag, _ := row["opnd_yn"].(string)
		if flag != "Y" && flag != "N" {
			return calendar.TradingDay{}, fmt.Errorf("invalid KIS opnd_yn")
		}
		if found != nil {
			return calendar.TradingDay{}, fmt.Errorf("duplicate KIS holiday date")
		}
		found = &calendar.TradingDay{BusinessDate: date, Open: flag == "Y", Source: "KIS chk-holiday opnd_yn", CheckedAt: cache.CheckedAt}
	}
	if found == nil {
		return calendar.TradingDay{}, fmt.Errorf("requested date absent from daily KIS holiday response")
	}
	return *found, nil
}
