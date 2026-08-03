package fileio

import (
	"path/filepath"
	"testing"

	"github.com/fifi/internal/testhelpers"
)

func TestWriteJSONAtomicAndReadCache(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "test.json")

	payload := map[string]string{"hello": "world"}
	if err := WriteJSONAtomic(targetPath, payload); err != nil {
		t.Fatalf("WriteJSONAtomic() error = %v", err)
	}

	// Read check
	raw, ok := ReadCacheFile(targetPath)
	if !ok {
		t.Fatalf("ReadCacheFile() failed to read written file")
	}

	if !bytesContains(raw, []byte(`"hello": "world"`)) {
		t.Errorf("ReadCacheFile() content = %s, expected to contain \"hello\": \"world\"", string(raw))
	}

	// Empty path checks
	if err := WriteJSONAtomic("", payload); err == nil {
		t.Errorf("WriteJSONAtomic(\"\") expected error, got nil")
	}
	if _, ok := ReadCacheFile(""); ok {
		t.Errorf("ReadCacheFile(\"\") expected ok=false, got true")
	}
}

func TestWriteCacheFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "test.bin")

	WriteCacheFile(targetPath, []byte("data"))
	raw, ok := ReadCacheFile(targetPath)
	if !ok || string(raw) != "data" {
		t.Errorf("WriteCacheFile/ReadCacheFile failed, got = %q (ok=%v)", string(raw), ok)
	}

	// Empty checks
	WriteCacheFile("", []byte("data"))
	WriteCacheFile(targetPath, nil)
}

func TestUnzipSingleFile(t *testing.T) {
	zipBytes, err := testhelpers.CreateMockZipArchive("inner.txt", []byte("hello inner"))
	if err != nil {
		t.Fatalf("CreateMockZipArchive error = %v", err)
	}

	extracted, err := UnzipSingleFile(zipBytes, "inner.txt")
	if err != nil {
		t.Fatalf("UnzipSingleFile() error = %v", err)
	}

	if string(extracted) != "hello inner" {
		t.Errorf("extracted content = %q, expected \"hello inner\"", string(extracted))
	}

	// Case insensitive check
	extracted2, err := UnzipSingleFile(zipBytes, "INNER.TXT")
	if err != nil {
		t.Fatalf("UnzipSingleFile() case insensitive error = %v", err)
	}
	if string(extracted2) != "hello inner" {
		t.Errorf("extracted case insensitive content = %q, expected \"hello inner\"", string(extracted2))
	}

	// Not found check
	_, err = UnzipSingleFile(zipBytes, "missing.txt")
	if err == nil {
		t.Errorf("UnzipSingleFile() expected error for missing file, got nil")
	}
}

func bytesContains(b, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(b)-len(sub); i++ {
		if equal(b[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
