package mstcache

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("Mstcache Cache Managers", func() {
	Context("EnsureZipCache", func() {
		It("downloads and extracts master files and reuses cached files on hit", func() {
			tmpDir := GinkgoT().TempDir()
			cachePath := filepath.Join(tmpDir, "cached_master.mst")

			zipBody, err := testhelpers.CreateMockZipArchive("master.mst", []byte("master content"))
			Expect(err).NotTo(HaveOccurred())

			transport := testhelpers.NewMockTransport()
			transport.New("https://example.test").
				Get("/master.zip").
				Reply(http.StatusOK).
				Body(zipBody)

			client := &http.Client{Transport: transport}

			// 1. Download and extract
			err = EnsureZipCache(context.Background(), client, "https://example.test/master.zip", "master.mst", cachePath)
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(cachePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("master content"))
			Expect(transport.Verify()).To(Succeed())

			// 2. Existing file should skip download
			transport.Reset()
			err = EnsureZipCache(context.Background(), client, "https://example.test/master.zip", "master.mst", cachePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(transport.Requests()).To(BeEmpty())
		})
	})

	Context("EnsureJSONSidecar", func() {
		It("generates JSON sidecar when missing or stale", func() {
			tmpDir := GinkgoT().TempDir()
			mstPath := filepath.Join(tmpDir, "master.mst")
			jsonPath := filepath.Join(tmpDir, "master.json")

			Expect(os.WriteFile(mstPath, []byte("mst"), 0o644)).To(Succeed())

			// 1. JSON does not exist -> generated
			called := false
			err := EnsureJSONSidecar(mstPath, jsonPath, func() (any, error) {
				called = true
				return map[string]string{"generated": "yes"}, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(called).To(BeTrue())

			// 2. JSON is newer -> not generated
			called = false
			err = EnsureJSONSidecar(mstPath, jsonPath, func() (any, error) {
				called = true
				return map[string]string{"generated": "yes"}, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(called).To(BeFalse())

			// 3. MST modified -> JSON is older -> generated again
			called = false
			past := time.Now().Add(-1 * time.Hour)
			Expect(os.Chtimes(jsonPath, past, past)).To(Succeed())

			err = EnsureJSONSidecar(mstPath, jsonPath, func() (any, error) {
				called = true
				return map[string]string{"generated": "yes"}, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(called).To(BeTrue())
		})
	})
})
