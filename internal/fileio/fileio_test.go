package fileio

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("FileIO Utilities", func() {
	Context("WriteJSONAtomic and ReadCacheFile", func() {
		It("writes JSON payload atomically and reads cache file", func() {
			tmpDir := GinkgoT().TempDir()
			targetPath := filepath.Join(tmpDir, "test.json")

			payload := map[string]string{"hello": "world"}
			Expect(WriteJSONAtomic(targetPath, payload)).To(Succeed())

			raw, ok := ReadCacheFile(targetPath)
			Expect(ok).To(BeTrue())
			Expect(string(raw)).To(ContainSubstring(`"hello": "world"`))

			// Empty path checks
			Expect(WriteJSONAtomic("", payload)).To(HaveOccurred())
			_, ok = ReadCacheFile("")
			Expect(ok).To(BeFalse())
		})
	})

	Context("WriteCacheFile", func() {
		It("writes cache byte content directly", func() {
			tmpDir := GinkgoT().TempDir()
			targetPath := filepath.Join(tmpDir, "test.bin")

			WriteCacheFile(targetPath, []byte("data"))
			raw, ok := ReadCacheFile(targetPath)
			Expect(ok).To(BeTrue())
			Expect(string(raw)).To(Equal("data"))

			// Empty checks
			WriteCacheFile("", []byte("data"))
			WriteCacheFile(targetPath, nil)
		})
	})

	Context("UnzipSingleFile", func() {
		It("extracts target file case-insensitively and returns error if missing", func() {
			zipBytes, err := testhelpers.CreateMockZipArchive("inner.txt", []byte("hello inner"))
			Expect(err).NotTo(HaveOccurred())

			extracted, err := UnzipSingleFile(zipBytes, "inner.txt")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(extracted)).To(Equal("hello inner"))

			// Case insensitive check
			extracted2, err := UnzipSingleFile(zipBytes, "INNER.TXT")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(extracted2)).To(Equal("hello inner"))

			// Not found check
			_, err = UnzipSingleFile(zipBytes, "missing.txt")
			Expect(err).To(HaveOccurred())
		})
	})
})
