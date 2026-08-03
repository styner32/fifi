package mstcache

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fifi/internal/testhelpers"
)

func TestEnsureZipCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cached_master.mst")

	zipBody, err := testhelpers.CreateMockZipArchive("master.mst", []byte("master content"))
	if err != nil {
		t.Fatalf("failed to create zip archive: %v", err)
	}

	transport := testhelpers.NewMockTransport()
	transport.New("https://example.test").
		Get("/master.zip").
		Reply(http.StatusOK).
		Body(zipBody)

	client := &http.Client{Transport: transport}

	// 1. Download and extract
	err = EnsureZipCache(context.Background(), client, "https://example.test/master.zip", "master.mst", cachePath)
	if err != nil {
		t.Fatalf("EnsureZipCache() error = %v", err)
	}

	content, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("failed to read cache path: %v", err)
	}
	if string(content) != "master content" {
		t.Errorf("cached content = %q, expected \"master content\"", string(content))
	}

	// Verify transport call
	if err := transport.Verify(); err != nil {
		t.Fatal(err)
	}

	// 2. Existing file should skip download
	transport.Reset()
	err = EnsureZipCache(context.Background(), client, "https://example.test/master.zip", "master.mst", cachePath)
	if err != nil {
		t.Fatalf("EnsureZipCache() error = %v", err)
	}
	if len(transport.Requests()) != 0 {
		t.Errorf("expected no HTTP requests on cache hit, got %d", len(transport.Requests()))
	}
}

func TestEnsureJSONSidecar(t *testing.T) {
	tmpDir := t.TempDir()
	mstPath := filepath.Join(tmpDir, "master.mst")
	jsonPath := filepath.Join(tmpDir, "master.json")

	if err := os.WriteFile(mstPath, []byte("mst"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. JSON does not exist -> generated
	called := false
	err := EnsureJSONSidecar(mstPath, jsonPath, func() (any, error) {
		called = true
		return map[string]string{"generated": "yes"}, nil
	})
	if err != nil {
		t.Fatalf("EnsureJSONSidecar() error = %v", err)
	}
	if !called {
		t.Errorf("expected build to be called")
	}

	// 2. JSON is newer -> not generated
	called = false
	// Wait a bit to ensure timestamp differences if files were recreated,
	// but since we aren't changing mstPath, JSON is already newer (created after mstPath).
	err = EnsureJSONSidecar(mstPath, jsonPath, func() (any, error) {
		called = true
		return map[string]string{"generated": "yes"}, nil
	})
	if err != nil {
		t.Fatalf("EnsureJSONSidecar() error = %v", err)
	}
	if called {
		t.Errorf("expected build not to be called when JSON is up-to-date")
	}

	// 3. MST modified -> JSON is older -> generated again
	called = false
	// Set JSON modtime to past
	past := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(jsonPath, past, past); err != nil {
		t.Fatal(err)
	}
	err = EnsureJSONSidecar(mstPath, jsonPath, func() (any, error) {
		called = true
		return map[string]string{"generated": "yes"}, nil
	})
	if err != nil {
		t.Fatalf("EnsureJSONSidecar() error = %v", err)
	}
	if !called {
		t.Errorf("expected build to be called after master modtime updated")
	}
}
