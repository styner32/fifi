package domesticfutureoption

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fifi/internal/envcfg"
	"github.com/fifi/internal/mstcache"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func (s *Service) LoadIndexFutureMaster(ctx context.Context, businessDate string) ([]MasterRecord, error) {
	cachePath := resolveIndexFutureMasterCachePath(envcfg.Get(indexFutureMasterCacheEnvKey, defaultIndexFutureMasterCache), businessDate)
	err := mstcache.EnsureZipCache(ctx, s.client, indexFutureMasterDownloadURL, indexFutureMasterFilename, cachePath)
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index future master cache: %w", err)
	}

	records, err := parseIndexFutureMaster(raw)
	if err != nil {
		return nil, err
	}

	jsonPath := resolveIndexFutureMasterJSONPath(cachePath)
	err = mstcache.EnsureJSONSidecar(cachePath, jsonPath, func() (any, error) {
		return masterJSONCache{
			BusinessDate: normalizeBusinessDate(businessDate),
			GeneratedAt:  time.Now().Format(time.RFC3339),
			SourcePath:   cachePath,
			RecordCount:  len(records),
			Fields: []masterJSONField{
				{Name: "info_type", Description: "1=지수선물, 5=지수콜옵션, 6=지수풋옵션 등"},
				{Name: "short_code", Description: "단축 종목코드"},
				{Name: "standard_code", Description: "표준 종목코드"},
				{Name: "name", Description: "한글 종목명"},
				{Name: "month_class_code", Description: "1=최근월물, 2=차근월물"},
				{Name: "underlying_short_code", Description: "기초자산 단축코드"},
				{Name: "underlying_name", Description: "기초자산명"},
			},
			Records: records,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	return records, nil
}

func parseIndexFutureMaster(raw []byte) ([]MasterRecord, error) {
	reader := transform.NewReader(bytes.NewReader(raw), korean.EUCKR.NewDecoder())
	scanner := bufio.NewScanner(reader)

	records := make([]MasterRecord, 0, 512)
	for scanner.Scan() {
		record, ok := parseIndexFutureMasterLine(scanner.Text())
		if !ok {
			continue
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, errors.New("index future master did not contain any parsable records")
	}

	return records, nil
}

func parseIndexFutureMasterLine(line string) (MasterRecord, bool) {
	parts := strings.Split(strings.TrimRight(line, "\r\n"), "|")
	if len(parts) < 9 {
		return MasterRecord{}, false
	}

	record := MasterRecord{
		InfoType:            strings.TrimSpace(parts[0]),
		ShortCode:           strings.TrimSpace(parts[1]),
		StandardCode:        strings.TrimSpace(parts[2]),
		Name:                strings.TrimSpace(parts[3]),
		ATMClassCode:        strings.TrimSpace(parts[4]),
		StrikePrice:         strings.TrimSpace(parts[5]),
		MonthClassCode:      strings.TrimSpace(parts[6]),
		UnderlyingShortCode: strings.TrimSpace(parts[7]),
		UnderlyingName:      strings.TrimSpace(parts[8]),
	}

	if record.ShortCode == "" || record.Name == "" {
		return MasterRecord{}, false
	}

	return record, true
}

func resolveIndexFutureMasterCachePath(cachePath string, businessDate string) string {
	return resolveBusinessDateCachePath(cachePath, businessDate, defaultIndexFutureMasterCache)
}

func resolveIndexFutureMasterJSONPath(cachePath string) string {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		cachePath = defaultIndexFutureMasterCache
	}

	ext := filepath.Ext(cachePath)
	if ext == "" {
		return cachePath + ".json"
	}
	return strings.TrimSuffix(cachePath, ext) + ".json"
}

func resolveBusinessDateCachePath(cachePath string, businessDate string, defaultPath string) string {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		cachePath = defaultPath
	}

	businessDate = normalizeBusinessDate(businessDate)
	ext := filepath.Ext(cachePath)
	if ext == "" {
		return cachePath + "." + businessDate
	}

	base := strings.TrimSuffix(cachePath, ext)
	return base + "." + businessDate + ext
}

func normalizeBusinessDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 8 {
		return time.Now().Format("20060102")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return time.Now().Format("20060102")
		}
	}
	return value
}


