package filings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fifi/internal/external/dart"
	"github.com/fifi/internal/kst"
)

// DefaultCorpCodeCacheFile is the base path for the dated DART corp code cache.
// The resolved path is .cache/dart_corp_code.<YYYYMMDD>.json.
const DefaultCorpCodeCacheFile = ".cache/dart_corp_code.json"

// CorpCodeCacheEnvKey overrides DefaultCorpCodeCacheFile (still a base path).
const CorpCodeCacheEnvKey = "DART_CORP_CODE_CACHE_FILE"

// CompanyLister is the subset of the DART client used for corp code lookup.
type CompanyLister interface {
	GetCompanies() ([]dart.Company, error)
}

// CorpCodeEntry maps a listed ticker to its DART corp code.
type CorpCodeEntry struct {
	StockCode string `json:"stock_code"`
	CorpCode  string `json:"corp_code"`
	CorpName  string `json:"corp_name"`
}

type corpCodeCache struct {
	BusinessDate string          `json:"business_date"`
	GeneratedAt  time.Time       `json:"generated_at"`
	Source       string          `json:"source"`
	RecordCount  int             `json:"record_count"`
	Records      []CorpCodeEntry `json:"records"`
}

// ResolveCorpCodeCachePath turns a base path into a date-scoped cache path.
func ResolveCorpCodeCachePath(base, date string) string {
	if strings.TrimSpace(base) == "" {
		base = DefaultCorpCodeCacheFile
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".json"
	}
	return fmt.Sprintf("%s.%s%s", stem, date, ext)
}

// CorpCodeResolver resolves stock codes to DART corp codes using corpCode.xml.
// DART publishes a full list (~100k corps); only listed corps are cached, once
// per KST date, so repeated daemon ticks do not re-download it.
type CorpCodeResolver struct {
	Lister        CompanyLister
	CacheBasePath string
	Now           func() time.Time

	loadedDate string
	byStock    map[string]CorpCodeEntry
}

// NewCorpCodeResolver builds a resolver using DART_CORP_CODE_CACHE_FILE or the default base path.
func NewCorpCodeResolver(lister CompanyLister) *CorpCodeResolver {
	base := strings.TrimSpace(os.Getenv(CorpCodeCacheEnvKey))
	if base == "" {
		base = DefaultCorpCodeCacheFile
	}
	return &CorpCodeResolver{Lister: lister, CacheBasePath: base, Now: kst.Now}
}

func (r *CorpCodeResolver) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return kst.Now()
}

func (r *CorpCodeResolver) load() (map[string]CorpCodeEntry, error) {
	date := r.now().In(kst.Location).Format("20060102")
	if r.byStock != nil && r.loadedDate == date {
		return r.byStock, nil
	}
	path := ResolveCorpCodeCachePath(r.CacheBasePath, date)

	if raw, err := os.ReadFile(path); err == nil {
		var cache corpCodeCache
		if err := json.Unmarshal(raw, &cache); err == nil && len(cache.Records) > 0 {
			r.setIndex(date, cache.Records)
			return r.byStock, nil
		}
	}

	if r.Lister == nil {
		return nil, errors.New("corp code resolver: no DART client configured")
	}
	companies, err := r.Lister.GetCompanies()
	if err != nil {
		return nil, fmt.Errorf("corp code resolver: fetch corpCode.xml: %w", err)
	}
	var records []CorpCodeEntry
	for _, c := range companies {
		stock := strings.ToUpper(strings.TrimSpace(c.StockCode))
		if stock == "" {
			continue
		}
		records = append(records, CorpCodeEntry{StockCode: stock, CorpCode: strings.TrimSpace(c.CorpCode), CorpName: strings.TrimSpace(c.CorpName)})
	}
	if len(records) == 0 {
		return nil, errors.New("corp code resolver: corpCode.xml had no listed companies")
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StockCode < records[j].StockCode })

	cache := corpCodeCache{BusinessDate: date, GeneratedAt: time.Now(), Source: "opendart corpCode.xml", RecordCount: len(records), Records: records}
	if err := writeJSONFile(path, cache); err != nil {
		// Cache write failure only costs a re-download; keep going.
		fmt.Fprintf(os.Stderr, "[filings] warning: write corp code cache %s: %v\n", path, err)
	}
	r.setIndex(date, records)
	return r.byStock, nil
}

func (r *CorpCodeResolver) setIndex(date string, records []CorpCodeEntry) {
	r.byStock = make(map[string]CorpCodeEntry, len(records))
	for _, rec := range records {
		r.byStock[rec.StockCode] = rec
	}
	r.loadedDate = date
}

// Resolve fills CorpCode (and Name when empty) for each target. Targets that
// already carry a corp code are trusted as-is and do not trigger a download.
// Unresolvable targets are returned separately so callers can log and skip them.
func (r *CorpCodeResolver) Resolve(targets []Target) (resolved []Target, unresolved []Target, err error) {
	needLookup := false
	for _, t := range targets {
		if t.CorpCode == "" {
			needLookup = true
			break
		}
	}
	var index map[string]CorpCodeEntry
	if needLookup {
		index, err = r.load()
		if err != nil {
			return nil, nil, err
		}
	}
	for _, t := range targets {
		if t.CorpCode == "" {
			entry, ok := index[t.StockCode]
			if !ok || entry.CorpCode == "" {
				unresolved = append(unresolved, t)
				continue
			}
			t.CorpCode = entry.CorpCode
			if t.Name == "" {
				t.Name = entry.CorpName
			}
		}
		resolved = append(resolved, t)
	}
	return resolved, unresolved, nil
}

func writeJSONFile(path string, v any) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

