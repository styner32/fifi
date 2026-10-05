package domesticstock

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

const KOSPIIndexMasterUniverse = "KIS_KOSPI_FLAG_Y_COMMON_CLASS_0"
const KOSPIIndexWeightStatus = "PROXY_NOT_OFFICIAL_INDEX_WEIGHTS"

// KOSPIIndexMarketCapSummary separates the index-oriented universe from the
// market-cap summary used by market-wide valuation reports. Membership is a KIS
// master flag, not an independently reconciled KRX constituent/weight file.
func (s *Service) KOSPIIndexMarketCapSummary(ctx context.Context, date string) (*KOSPIMarketCapSummary, error) {
	if _, e := time.Parse("20060102", date); e != nil {
		return nil, fmt.Errorf("invalid business date")
	}
	if _, e := s.loadKOSPIMaster(ctx, date); e != nil {
		return nil, e
	}
	base := strings.TrimSpace(os.Getenv(kospiMasterCacheEnvKey))
	if base == "" {
		base = defaultKOSPIMasterCache
	}
	path := resolveKOSPIMasterCachePath(base, date)
	raw, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	result, e := buildKOSPIIndexMarketCapSummary(raw, date)
	if e != nil {
		return nil, e
	}
	result.SourcePath = path
	return result, nil
}

func buildKOSPIIndexMarketCapSummary(raw []byte, date string) (*KOSPIMarketCapSummary, error) {
	result := &KOSPIMarketCapSummary{BusinessDate: date, Universe: KOSPIIndexMasterUniverse, WeightStatus: KOSPIIndexWeightStatus}
	sc := bufio.NewScanner(transform.NewReader(bytes.NewReader(raw), korean.EUCKR.NewDecoder()))
	seen := map[string]bool{}
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		if len([]rune(strings.TrimRight(text, "\r\n"))) < kospiPart2Width+21 {
			return nil, fmt.Errorf("truncated master line %d", line)
		}
		r, ok, e := parseKOSPIMasterLine(text)
		if e != nil {
			return nil, e
		}
		if !r.KOSPIIndexMember {
			continue
		}
		if r.PreferredClass != "0" {
			return nil, fmt.Errorf("conflicting/unknown common-share classification: %s", r.Code)
		}
		if !ok || r.MarketCap <= 0 || math.IsNaN(r.MarketCap) || math.IsInf(r.MarketCap, 0) {
			return nil, fmt.Errorf("missing index-universe market cap: %s", r.Code)
		}
		if seen[r.Code] {
			return nil, fmt.Errorf("duplicate index constituent: %s", r.Code)
		}
		seen[r.Code] = true
		result.TotalMarketCap += r.MarketCap
		result.Constituents = append(result.Constituents, KOSPIMarketCapConstituent{Code: r.Code, Name: r.Name, MarketCap: r.MarketCap, BaseDate: r.BaseDate, KOSPIIndexMember: true, PreferredClass: "0"})
	}
	if e := sc.Err(); e != nil {
		return nil, e
	}
	if len(result.Constituents) == 0 {
		return nil, fmt.Errorf("empty index master universe")
	}
	sort.Slice(result.Constituents, func(i, j int) bool {
		a, b := result.Constituents[i], result.Constituents[j]
		if a.MarketCap == b.MarketCap {
			return a.Code < b.Code
		}
		return a.MarketCap > b.MarketCap
	})
	return result, nil
}
