package mstcache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/fifi/internal/fileio"
)

// Doer represents an HTTP client capable of making a request.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// EnsureZipCache downloads a zip archive from url, extracts targetName, and atomically caches it to cachePath.
func EnsureZipCache(ctx context.Context, client Doer, url, targetName, cachePath string) error {
	_, err := os.Stat(cachePath)
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat cache path: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed (status %d)", resp.StatusCode)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	masterBytes, err := fileio.UnzipSingleFile(zipBytes, targetName)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, masterBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace cache: %w", err)
	}

	return nil
}

// EnsureJSONSidecar checks if jsonPath exists and is newer than mstPath. If not, it builds the payload using build() and writes it.
func EnsureJSONSidecar(mstPath, jsonPath string, build func() (any, error)) error {
	masterInfo, err := os.Stat(mstPath)
	if err != nil {
		return fmt.Errorf("failed to stat master path: %w", err)
	}

	if jsonInfo, err := os.Stat(jsonPath); err == nil && !masterInfo.ModTime().After(jsonInfo.ModTime()) {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat json path: %w", err)
	}

	payload, err := build()
	if err != nil {
		return err
	}

	return fileio.WriteJSONAtomic(jsonPath, payload)
}
