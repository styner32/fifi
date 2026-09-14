package domesticstock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *Service) KOSPIProxyPBR(ctx context.Context, targetCoverage float64) (*ProxyPBRResult, error) {
	if targetCoverage <= 0 || targetCoverage > 1 {
		return nil, errors.New("targetCoverage must be > 0 and <= 1")
	}

	records, err := s.loadKOSPIMaster(ctx, time.Now().Format("20060102"))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("kospi master file did not contain usable records")
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].MarketCap > records[j].MarketCap
	})

	totalMarketCap := 0.0
	for _, record := range records {
		totalMarketCap += record.MarketCap
	}
	if totalMarketCap <= 0 {
		return nil, errors.New("total KOSPI market cap is zero")
	}

	scale, calibrationSymbol, err := s.resolveKOSPIProxyScale(ctx, records)
	if err != nil {
		return nil, err
	}

	targetMarketCap := totalMarketCap * targetCoverage

	result := &ProxyPBRResult{
		Market:            "KOSPI",
		Method:            "master-file-proxy",
		TargetCoverage:    targetCoverage,
		TotalCount:        len(records),
		TotalMarketCap:    totalMarketCap,
		EarningsScale:     scale,
		CalibrationSymbol: calibrationSymbol,
	}

	for _, record := range records {
		result.SelectedCount++
		result.RawSelectedCap += record.MarketCap

		bookEquity, stockProxyPBR, ok := proxyBookEquity(record, scale)
		if !ok {
			result.SkippedCount++
			result.SkippedMarketCap += record.MarketCap
		} else {
			result.UsedCount++
			result.UsedMarketCap += record.MarketCap
			result.AggregateBookValue += bookEquity
			result.Constituents = append(result.Constituents, ProxyPBRConstituent{
				Code:       record.Code,
				Name:       record.Name,
				MarketCap:  record.MarketCap,
				ROE:        record.ROE,
				NetIncome:  record.NetIncome * scale,
				BookEquity: bookEquity,
				ProxyPBR:   stockProxyPBR,
				Coverage:   record.MarketCap / totalMarketCap,
				BaseDate:   record.BaseDate,
			})
			if result.BasisDate == "" {
				result.BasisDate = record.BaseDate
			}
		}

		if result.UsedMarketCap >= targetMarketCap {
			break
		}
	}

	if result.AggregateBookValue <= 0 {
		return nil, errors.New("aggregate book value is zero; unable to compute proxy PBR")
	}

	result.RawCoverage = result.RawSelectedCap / totalMarketCap
	result.UsedCoverage = result.UsedMarketCap / totalMarketCap
	result.ProxyPBR = result.UsedMarketCap / result.AggregateBookValue

	return result, nil
}

func (s *Service) KOSPIActualPBR(ctx context.Context, targetCoverage float64, businessDate string) (*ActualPBRResult, error) {
	if targetCoverage <= 0 || targetCoverage > 1 {
		return nil, errors.New("targetCoverage must be > 0 and <= 1")
	}

	startedAt := time.Now()
	businessDate = strings.TrimSpace(businessDate)
	if businessDate == "" {
		businessDate = time.Now().Format("20060102")
	}
	debug := actualPBRDebugEnabled()
	progress := actualPBRProgressEnabled()
	actualPBRProgressLog(progress, "start target=%.2f%% business_date=%s", targetCoverage*100, businessDate)
	actualPBRLog(debug, "start target=%.2f%% business_date=%s", targetCoverage*100, businessDate)

	masterStartedAt := time.Now()
	records, err := s.loadKOSPIMaster(ctx, businessDate)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("kospi master file did not contain usable records")
	}
	masterLoadTime := time.Since(masterStartedAt)
	actualPBRProgressLog(progress, "master loaded records=%d took=%s", len(records), masterLoadTime)
	actualPBRLog(debug, "master loaded records=%d took=%s", len(records), masterLoadTime)

	sort.Slice(records, func(i, j int) bool {
		return records[i].MarketCap > records[j].MarketCap
	})

	totalMarketCap := 0.0
	for _, record := range records {
		totalMarketCap += record.MarketCap
	}
	if totalMarketCap <= 0 {
		return nil, errors.New("total KOSPI market cap is zero")
	}

	cachePath := strings.TrimSpace(os.Getenv(kospiActualPBRCacheEnvKey))
	if cachePath == "" {
		cachePath = defaultKOSPIActualPBRCache
	}
	cachePath = resolveKOSPIActualPBRCachePath(cachePath, businessDate)
	masterCachePath := resolveKOSPIMasterCachePath(getOrDefaultEnv(kospiMasterCacheEnvKey, defaultKOSPIMasterCache), businessDate)

	cacheStartedAt := time.Now()
	cache, err := loadActualPBRCache(cachePath)
	if err != nil {
		return nil, err
	}
	cacheLoadTime := time.Since(cacheStartedAt)
	actualPBRProgressLog(progress, "cache loaded entries=%d path=%s took=%s", len(cache), cachePath, cacheLoadTime)
	actualPBRLog(debug, "cache loaded entries=%d path=%s took=%s", len(cache), cachePath, cacheLoadTime)

	targetMarketCap := totalMarketCap * targetCoverage

	result := &ActualPBRResult{
		Market:             "KOSPI",
		Method:             "master-cap-actual-pbr",
		TargetCoverage:     targetCoverage,
		TotalCount:         len(records),
		TotalMarketCap:     totalMarketCap,
		BusinessDate:       businessDate,
		ActualPBRCachePath: cachePath,
		MasterCachePath:    masterCachePath,
		MasterJSONPath:     resolveKOSPIMasterJSONPath(masterCachePath),
		MasterLoadTime:     masterLoadTime,
		CacheLoadTime:      cacheLoadTime,
	}

	for _, record := range records {
		result.SelectedCount++
		result.RawSelectedCap += record.MarketCap

		lookup, lookupErr := s.lookupActualPBR(ctx, record, businessDate, cache, cachePath, progress)
		result.RateLimitWaitTime += lookup.WaitDuration
		result.PriceFetchTime += lookup.FetchDuration
		result.CacheSaveTime += lookup.CacheSaveTime
		if lookupErr != nil || lookup.PBR <= 0 {
			result.SkippedCount++
			result.SkippedMarketCap += record.MarketCap
			actualPBRLog(debug, "skip code=%s name=%s err=%v", record.Code, record.Name, lookupErr)
			continue
		}

		pbr := lookup.PBR
		effectiveMarketCap := lookup.MarketCap
		if effectiveMarketCap <= 0 {
			effectiveMarketCap = record.MarketCap
		}

		bookEquity := effectiveMarketCap / pbr
		if bookEquity <= 0 || math.IsNaN(bookEquity) || math.IsInf(bookEquity, 0) {
			result.SkippedCount++
			result.SkippedMarketCap += record.MarketCap
			continue
		}

		if lookup.CacheHit {
			result.CacheHitCount++
		} else {
			result.APICallCount++
		}

		result.UsedCount++
		result.UsedMarketCap += effectiveMarketCap
		result.AggregateBookValue += bookEquity
		result.Constituents = append(result.Constituents, ActualPBRConstituent{
			Code:       record.Code,
			Name:       record.Name,
			MarketCap:  effectiveMarketCap,
			PBR:        pbr,
			BookEquity: bookEquity,
			Coverage:   effectiveMarketCap / totalMarketCap,
			BaseDate:   record.BaseDate,
			CacheHit:   lookup.CacheHit,
		})

		if progress && (!lookup.CacheHit || result.SelectedCount%10 == 0 || result.UsedMarketCap >= targetMarketCap) {
			actualPBRProgressLog(
				progress,
				"progress selected=%d used=%d cache=%d api=%d raw=%.2f%% used=%.2f%% last=%s",
				result.SelectedCount,
				result.UsedCount,
				result.CacheHitCount,
				result.APICallCount,
				(result.RawSelectedCap/totalMarketCap)*100,
				(result.UsedMarketCap/totalMarketCap)*100,
				record.Code,
			)
		}

		if debug && (!lookup.CacheHit || result.SelectedCount%10 == 0 || result.UsedMarketCap >= targetMarketCap) {
			actualPBRLog(
				debug,
				"progress selected=%d used=%d cache=%d api=%d raw=%.2f%% used=%.2f%% last=%s wait=%s fetch=%s",
				result.SelectedCount,
				result.UsedCount,
				result.CacheHitCount,
				result.APICallCount,
				(result.RawSelectedCap/totalMarketCap)*100,
				(result.UsedMarketCap/totalMarketCap)*100,
				record.Code,
				lookup.WaitDuration,
				lookup.FetchDuration,
			)
		}

		if result.UsedMarketCap >= targetMarketCap {
			break
		}
	}

	if result.AggregateBookValue <= 0 {
		return nil, errors.New("aggregate book value is zero; unable to compute weighted PBR")
	}

	result.RawCoverage = result.RawSelectedCap / totalMarketCap
	result.UsedCoverage = result.UsedMarketCap / totalMarketCap
	result.WeightedPBR = result.UsedMarketCap / result.AggregateBookValue
	result.TotalDuration = time.Since(startedAt)
	actualPBRProgressLog(
		progress,
		"done weighted_pbr=%.4f used=%.2f%% cache=%d api=%d total=%s",
		result.WeightedPBR,
		result.UsedCoverage*100,
		result.CacheHitCount,
		result.APICallCount,
		result.TotalDuration,
	)
	actualPBRLog(
		debug,
		"done weighted_pbr=%.4f used=%.2f%% cache=%d api=%d wait=%s fetch=%s total=%s",
		result.WeightedPBR,
		result.UsedCoverage*100,
		result.CacheHitCount,
		result.APICallCount,
		result.RateLimitWaitTime,
		result.PriceFetchTime,
		result.TotalDuration,
	)

	if result.UsedCoverage < targetCoverage {
		return nil, fmt.Errorf(
			"insufficient KOSPI actual PBR coverage: used %.2f%% target %.2f%%",
			result.UsedCoverage*100,
			targetCoverage*100,
		)
	}

	return result, nil
}

func (s *Service) lookupActualPBR(
	ctx context.Context,
	record kospiMasterRecord,
	businessDate string,
	cache actualPBRCache,
	cachePath string,
	progress bool,
) (actualPBRLookupResult, error) {
	if entry, ok := cache[record.Code]; ok && entry.BusinessDate == businessDate && entry.PBR > 0 {
		return actualPBRLookupResult{
			PBR:       entry.PBR,
			MarketCap: entry.MarketCap,
			CacheHit:  true,
		}, nil
	}

	actualPBRProgressLog(progress, "fetch start code=%s name=%s", record.Code, record.Name)
	fetchStartedAt := time.Now()
	resp, err := s.InquirePrice(ctx, record.Code)
	fetchDuration := time.Since(fetchStartedAt)
	if err != nil {
		return actualPBRLookupResult{
			FetchDuration: fetchDuration,
		}, err
	}
	if !resp.IsOK() {
		return actualPBRLookupResult{
			FetchDuration: fetchDuration,
		}, fmt.Errorf("inquire-price error for %s: msg_cd=%s msg1=%s", record.Code, resp.MessageCode(), resp.Message())
	}

	row := firstOutputRow(resp, "output")
	if row == nil {
		return actualPBRLookupResult{
			FetchDuration: fetchDuration,
		}, fmt.Errorf("inquire-price output missing for %s", record.Code)
	}

	pbr, ok := parseFloat(row["pbr"])
	if !ok || pbr <= 0 {
		return actualPBRLookupResult{
			FetchDuration: fetchDuration,
		}, fmt.Errorf("invalid pbr for %s", record.Code)
	}

	effectiveMarketCap := record.MarketCap
	if actualMarketCap, ok := parseFloat(row["hts_avls"]); ok && actualMarketCap > 0 {
		effectiveMarketCap = actualMarketCap
	}

	cacheSaveStartedAt := time.Now()
	cache[record.Code] = actualPBRCacheEntry{
		PBR:          pbr,
		MarketCap:    effectiveMarketCap,
		BusinessDate: businessDate,
		UpdatedAt:    time.Now().Format(time.RFC3339),
		Name:         record.Name,
	}

	if err := saveActualPBRCache(cachePath, cache); err != nil {
		return actualPBRLookupResult{
			FetchDuration: fetchDuration,
		}, err
	}
	cacheSaveTime := time.Since(cacheSaveStartedAt)

	return actualPBRLookupResult{
		PBR:           pbr,
		MarketCap:     effectiveMarketCap,
		CacheHit:      false,
		FetchDuration: fetchDuration,
		CacheSaveTime: cacheSaveTime,
	}, nil
}

func loadActualPBRCache(cachePath string) (actualPBRCache, error) {
	if strings.TrimSpace(cachePath) == "" {
		return actualPBRCache{}, nil
	}

	raw, err := os.ReadFile(cachePath)
	if errors.Is(err, os.ErrNotExist) {
		return actualPBRCache{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read actual PBR cache: %w", err)
	}
	if len(raw) == 0 {
		return actualPBRCache{}, nil
	}

	cache := actualPBRCache{}
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actual PBR cache: %w", err)
	}
	return cache, nil
}

func saveActualPBRCache(cachePath string, cache actualPBRCache) error {
	if strings.TrimSpace(cachePath) == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("failed to create actual PBR cache dir: %w", err)
	}

	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal actual PBR cache: %w", err)
	}

	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return fmt.Errorf("failed to write actual PBR cache temp file: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return fmt.Errorf("failed to replace actual PBR cache: %w", err)
	}

	return nil
}

func (s *Service) resolveKOSPIProxyScale(ctx context.Context, records []kospiMasterRecord) (float64, string, error) {
	if raw := strings.TrimSpace(os.Getenv(kospiProxyScaleEnvKey)); raw != "" {
		scale, ok := parseFloat(raw)
		if !ok || scale <= 0 {
			return 0, "", fmt.Errorf("%s must be a positive number", kospiProxyScaleEnvKey)
		}
		return scale, "ENV", nil
	}

	for _, record := range records {
		if record.MarketCap <= 0 || record.NetIncome <= 0 || record.ROE <= 0 {
			continue
		}

		resp, err := s.InquirePrice(ctx, record.Code)
		if err != nil {
			continue
		}
		if !resp.IsOK() {
			continue
		}

		row := firstOutputRow(resp, "output")
		if row == nil {
			continue
		}

		pbr, ok := parseFloat(row["pbr"])
		if !ok || pbr <= 0 {
			continue
		}

		scale := (record.MarketCap * record.ROE) / (100 * record.NetIncome * pbr)
		if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			continue
		}

		return scale, record.Code, nil
	}

	return 0, "", errors.New("unable to calibrate KOSPI proxy PBR scale from inquire-price")
}

func proxyBookEquity(record kospiMasterRecord, scale float64) (float64, float64, bool) {
	if record.MarketCap <= 0 || record.NetIncome <= 0 || record.ROE <= 0 || scale <= 0 {
		return 0, 0, false
	}

	bookEquity := (record.NetIncome * scale) / (record.ROE / 100)
	if bookEquity <= 0 || math.IsNaN(bookEquity) || math.IsInf(bookEquity, 0) {
		return 0, 0, false
	}

	return bookEquity, record.MarketCap / bookEquity, true
}
