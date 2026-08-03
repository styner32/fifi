package fileio

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// WriteJSONAtomic writes the payload to a path using a temporary file and renaming it.
func WriteJSONAtomic(path string, payload any) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace file: %w", err)
	}
	return nil
}

// ReadCacheFile reads and returns bytes from a cache file. Returns false if missing/empty.
func ReadCacheFile(path string) ([]byte, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}

// WriteCacheFile writes raw bytes to path atomically.
func WriteCacheFile(path string, raw []byte) {
	path = strings.TrimSpace(path)
	if path == "" || len(raw) == 0 {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmpPath, path)
}

// UnzipSingleFile extracts targetName from zip bytes.
func UnzipSingleFile(zipBytes []byte, targetName string) ([]byte, error) {
	readerAt := bytes.NewReader(zipBytes)
	zipReader, err := zip.NewReader(readerAt, int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}

	for _, file := range zipReader.File {
		if !strings.EqualFold(file.Name, targetName) {
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer fileReader.Close()

		return io.ReadAll(fileReader)
	}

	return nil, fmt.Errorf("%s not found in zip archive", targetName)
}
