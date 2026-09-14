package domesticstock

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

	"github.com/fifi/internal/mstcache"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func (s *Service) loadKOSPIMaster(ctx context.Context, businessDate string) ([]kospiMasterRecord, error) {
	cachePath := strings.TrimSpace(os.Getenv(kospiMasterCacheEnvKey))
	if cachePath == "" {
		cachePath = defaultKOSPIMasterCache
	}
	cachePath = resolveKOSPIMasterCachePath(cachePath, businessDate)

	err := mstcache.EnsureZipCache(ctx, s.client, kospiMasterDownloadURL, kospiMasterFilename, cachePath)
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read KOSPI master cache: %w", err)
	}

	records, err := parseKOSPIMaster(raw)
	if err != nil {
		return nil, err
	}

	jsonPath := resolveKOSPIMasterJSONPath(cachePath)
	err = mstcache.EnsureJSONSidecar(cachePath, jsonPath, func() (any, error) {
		return kospiMasterJSONCache{
			BusinessDate: strings.TrimSpace(businessDate),
			GeneratedAt:  time.Now().Format(time.RFC3339),
			SourcePath:   cachePath,
			RecordCount:  len(records),
			Fields: []kospiMasterJSONField{
				{Name: "code", Description: "6자리 단축 종목코드", Source: "part1[0:9]"},
				{Name: "name", Description: "한글 종목명", Source: "part1[21:]"},
				{Name: "market_cap", Description: "전일기준 시가총액(억)", Source: "field[65]"},
				{Name: "net_income", Description: "당기순이익", Source: "field[62]"},
				{Name: "roe", Description: "ROE(자기자본이익률)", Source: "field[63]"},
				{Name: "base_date", Description: "재무 기준년월", Source: "field[64]"},
			},
			Records: records,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	return records, nil
}

func resolveKOSPIMasterCachePath(cachePath string, businessDate string) string {
	return resolveBusinessDateCachePath(cachePath, businessDate, defaultKOSPIMasterCache)
}

func resolveKOSPIActualPBRCachePath(cachePath string, businessDate string) string {
	return resolveBusinessDateCachePath(cachePath, businessDate, defaultKOSPIActualPBRCache)
}

func resolveBusinessDateCachePath(cachePath string, businessDate string, defaultPath string) string {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		cachePath = defaultPath
	}

	businessDate = strings.TrimSpace(businessDate)
	if !isYYYYMMDD(businessDate) {
		businessDate = time.Now().Format("20060102")
	}

	ext := filepath.Ext(cachePath)
	if ext == "" {
		return cachePath + "." + businessDate
	}

	base := strings.TrimSuffix(cachePath, ext)
	return base + "." + businessDate + ext
}

func resolveKOSPIMasterJSONPath(cachePath string) string {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		cachePath = defaultKOSPIMasterCache
	}

	ext := filepath.Ext(cachePath)
	if ext == "" {
		return cachePath + ".json"
	}

	return strings.TrimSuffix(cachePath, ext) + ".json"
}



func parseKOSPIMaster(raw []byte) ([]kospiMasterRecord, error) {
	reader := transform.NewReader(bytes.NewReader(raw), korean.EUCKR.NewDecoder())
	scanner := bufio.NewScanner(reader)

	records := make([]kospiMasterRecord, 0)
	for scanner.Scan() {
		record, ok, err := parseKOSPIMasterLine(scanner.Text())
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func parseKOSPIMasterLine(line string) (kospiMasterRecord, bool, error) {
	runes := []rune(strings.TrimRight(line, "\r\n"))
	if len(runes) < kospiPart2Width {
		return kospiMasterRecord{}, false, nil
	}

	part1 := runes[:len(runes)-kospiPart2Width]
	part2 := runes[len(runes)-kospiPart2Width:]
	if len(part1) < 21 {
		return kospiMasterRecord{}, false, nil
	}

	code := strings.TrimSpace(string(part1[:9]))
	name := strings.TrimSpace(string(part1[21:]))
	if code == "" || name == "" {
		return kospiMasterRecord{}, false, nil
	}

	fields, err := splitFixedWidthFields(part2, kospiPart2FieldWidths)
	if err != nil {
		return kospiMasterRecord{}, false, err
	}
	if len(fields) < 70 {
		return kospiMasterRecord{}, false, fmt.Errorf("unexpected KOSPI master field count: %d", len(fields))
	}

	isCommon := strings.ToUpper(strings.TrimSpace(fields[58])) == "Y"
	pref := strings.TrimSpace(fields[54])
	isPref := pref != "" && pref != "0" && strings.ToUpper(pref) != "N"
	if !isCommon && !isPref {
		return kospiMasterRecord{}, false, nil
	}

	marketCap, ok := parseFloat(fields[65])
	if !ok || marketCap <= 0 {
		return kospiMasterRecord{}, false, nil
	}

	netIncome, _ := parseFloat(fields[62])
	roe, _ := parseFloat(fields[63])

	return kospiMasterRecord{
		Code:      normalizeShortCode(code),
		Name:      name,
		MarketCap: marketCap,
		NetIncome: netIncome,
		ROE:       roe,
		BaseDate:  strings.TrimSpace(fields[64]),
	}, true, nil
}

func splitFixedWidthFields(runes []rune, widths []int) ([]string, error) {
	fields := make([]string, 0, len(widths))
	offset := 0
	for _, width := range widths {
		if offset+width > len(runes) {
			return nil, errors.New("fixed-width data shorter than expected")
		}
		fields = append(fields, strings.TrimSpace(string(runes[offset:offset+width])))
		offset += width
	}
	return fields, nil
}

func normalizeShortCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.TrimPrefix(code, "A")
	if len(code) > 6 {
		code = code[len(code)-6:]
	}
	return code
}


