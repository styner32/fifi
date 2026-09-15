package kofia

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fifi/internal/fileio"
)

// CacheEntry is the JSON structure for daily-cached KOFIA data.
type CacheEntry struct {
	SchemaVersion int              `json:"schema_version"`
	AmountUnit    string           `json:"amount_unit"`
	BusinessDate  string           `json:"business_date"`
	GeneratedAt   time.Time        `json:"generated_at"`
	RecordCount   int              `json:"record_count"`
	Rows          []MarketFundsRow `json:"rows"`
}

// CachedClient wraps Client with business-date-scoped caching.
// Cache path format: <cacheDir>/kofia_market_funds.<YYYYMMDD>.json
type CachedClient struct {
	client   *Client
	cacheDir string
}

// NewCachedClient creates a KOFIA client with daily caching.
func NewCachedClient(cacheDir, userAgent string) *CachedClient {
	return &CachedClient{
		client:   NewClient(userAgent),
		cacheDir: cacheDir,
	}
}

// GetMarketFundsForDate returns the latest row matching the given date.
// It checks cache first; if the dated cache file exists, it returns from cache.
// Otherwise it fetches from FreeSIS, caches, and returns.
func (cc *CachedClient) GetMarketFundsForDate(ctx context.Context, date string) (*MarketFundsRow, error) {
	date = strings.ReplaceAll(date, "-", "")
	if _, e := time.Parse("20060102", date); e != nil {
		return nil, fmt.Errorf("kofia: invalid date format %q, expected YYYYMMDD", date)
	}

	// Try cache first
	cachePath := cc.cachePath(date)
	if entry, err := cc.loadCache(cachePath); err == nil && entry.BusinessDate == date {
		row := cc.findRow(entry.Rows, date)
		if row != nil {
			return row, nil
		}
	}

	// Fetch from FreeSIS (query the surrounding 5 business days to be safe)
	startDate := shiftDate(date, -7)
	rows, err := cc.client.GetMarketFunds(ctx, startDate, date)
	if err != nil {
		return nil, fmt.Errorf("kofia fetch: %w", err)
	}

	// Save cache
	if saveErr := cc.saveCache(cachePath, date, rows); saveErr != nil {
		fmt.Fprintf(os.Stderr, "[kofia] cache save warning: %v\n", saveErr)
	}

	row := cc.findRow(rows, date)
	if row == nil {
		return nil, fmt.Errorf("kofia: no data found for date %s", date)
	}
	return row, nil
}

func (cc *CachedClient) cachePath(date string) string {
	return filepath.Join(cc.cacheDir, fmt.Sprintf("kofia_market_funds.%s.json", date))
}

func (cc *CachedClient) loadCache(path string) (*CacheEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	if entry.SchemaVersion != 2 || entry.AmountUnit != marketFundsAmountUnit {
		return nil, fmt.Errorf("kofia: legacy unit cache requires refresh")
	}
	for _, row := range entry.Rows {
		if row.AmountUnit != marketFundsAmountUnit {
			return nil, fmt.Errorf("kofia: cache row unit missing")
		}
	}
	return &entry, nil
}

func (cc *CachedClient) saveCache(path, date string, rows []MarketFundsRow) error {
	// Preserve unversioned caches before replacing their ambiguous units.
	if raw, e := os.ReadFile(path); e == nil {
		var old CacheEntry
		if json.Unmarshal(raw, &old) == nil && old.SchemaVersion != 2 {
			if e = os.WriteFile(fmt.Sprintf("%s.legacy.%d", path, time.Now().UnixNano()), raw, 0600); e != nil {
				return e
			}
		}
	}
	entry := CacheEntry{SchemaVersion: 2, AmountUnit: marketFundsAmountUnit,
		BusinessDate: date,
		GeneratedAt:  time.Now(),
		RecordCount:  len(rows),
		Rows:         rows,
	}
	return fileio.WriteJSONAtomic(path, entry)
}

// findRow finds a row whose date matches the given YYYYMMDD date.
// KOFIA API dates are already in YYYYMMDD format.
func (cc *CachedClient) findRow(rows []MarketFundsRow, date string) *MarketFundsRow {
	best := -1
	for i, r := range rows {
		if _, e := time.Parse("20060102", r.Date); e == nil && r.Date <= date && (best < 0 || r.Date > rows[best].Date) {
			best = i
		}
	}
	if best >= 0 {
		return &rows[best]
	}

	return nil
}

// shiftDate shifts a YYYYMMDD date by n days.
func shiftDate(date string, days int) string {
	t, err := time.Parse("20060102", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, days).Format("20060102")
}
