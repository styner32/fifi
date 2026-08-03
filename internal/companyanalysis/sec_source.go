package companyanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/fifi/internal/fileio"
)

func (s *Service) resolveSECTicker(ctx context.Context, symbol string) (string, string, error) {
	raw, err := s.fetchSECJSON(ctx, s.cfg.SECTickersURL, strings.TrimSpace(s.cfg.SECTickersCachePath))
	if err != nil {
		return "", "", err
	}
	var payload map[string]secTickerEntry
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", fmt.Errorf("failed to decode SEC ticker list: %w", err)
	}

	for _, entry := range payload {
		if strings.EqualFold(strings.TrimSpace(entry.Ticker), symbol) {
			return fmt.Sprintf("%010d", entry.CIK), strings.TrimSpace(entry.Title), nil
		}
	}
	return "", "", fmt.Errorf("SEC ticker mapping not found for %s", symbol)
}

func (s *Service) fetchCompanyFacts(ctx context.Context, cik string) (*secCompanyFactsResponse, error) {
	url := strings.TrimRight(s.cfg.SECCompanyFactsBase, "/") + "/CIK" + strings.TrimSpace(cik) + ".json"
	raw, err := s.fetchSECJSON(ctx, url, resolveCompanyFactsCachePath(s.cfg.SECCompanyFactsCachePath, cik))
	if err != nil {
		return nil, err
	}
	var payload secCompanyFactsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode SEC company facts: %w", err)
	}
	return &payload, nil
}

func (s *Service) fetchSECJSON(ctx context.Context, url string, cachePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", s.cfg.SECUserAgent)

	resp, err := s.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			err = readErr
		} else if resp.StatusCode == http.StatusOK && json.Valid(raw) && !isSECFairAccessBody(raw) {
			writeCacheFile(cachePath, raw)
			return raw, nil
		} else if cached, ok := readCacheFile(cachePath); ok {
			return cached, nil
		} else if isSECFairAccessBody(raw) {
			return nil, fmt.Errorf("SEC blocked automated access. Set COMPANY_ANALYSIS_SEC_USER_AGENT to a descriptive identifier with contact email and retry")
		} else if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("SEC request failed with status %d", resp.StatusCode)
		} else {
			return nil, fmt.Errorf("SEC response was not valid JSON")
		}
	}

	if cached, ok := readCacheFile(cachePath); ok {
		return cached, nil
	}
	if err != nil {
		return nil, fmt.Errorf("SEC request failed: %w", err)
	}
	return nil, fmt.Errorf("SEC request failed")
}

func isSECFairAccessBody(raw []byte) bool {
	body := strings.ToLower(strings.TrimSpace(string(raw)))
	return strings.Contains(body, "automated access to our sites") &&
		strings.Contains(body, "privacy and security policy")
}

func readCacheFile(path string) ([]byte, bool) {
	return fileio.ReadCacheFile(path)
}

func writeCacheFile(path string, raw []byte) {
	fileio.WriteCacheFile(path, raw)
}

func resolveCompanyFactsCachePath(basePath string, cik string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return ""
	}
	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	stem := filename
	if ext != "" {
		stem = strings.TrimSuffix(filename, ext)
	}
	if ext == "" {
		ext = ".json"
	}
	cik = strings.TrimSpace(cik)
	if cik != "" {
		stem += "." + cik
	}
	return filepath.Join(dir, stem+ext)
}
