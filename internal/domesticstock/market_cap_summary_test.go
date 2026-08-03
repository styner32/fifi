package domesticstock

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KOSPIMarketCapSummary", func() {
	It("loads dated KOSPI master cache and exposes sorted market cap summary", func() {
		cacheDir := GinkgoT().TempDir()
		baseCachePath := filepath.Join(cacheDir, "kospi_code.mst")
		Expect(os.Setenv(kospiMasterCacheEnvKey, baseCachePath)).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(kospiMasterCacheEnvKey)).To(Succeed())
		})

		masterBody := strings.Join([]string{
			buildKOSPIMasterLine("A000001", "ALPHA", "Y", "100", "20", "20250331", "100"),
			buildKOSPIMasterLine("A000002", "BETA", "Y", "50", "20", "20250331", "300"),
		}, "")
		zipBody, err := testhelpers.CreateMockZipArchive(kospiMasterFilename, []byte(masterBody))
		Expect(err).NotTo(HaveOccurred())

		transport := testhelpers.NewMockTransport()
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/kospi_code.mst.zip").
			Reply(http.StatusOK).
			Body(zipBody)

		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.Client = &http.Client{Transport: transport}
		summary, err := NewService(client).KOSPIMarketCapSummary(context.Background(), "20260515")
		Expect(err).NotTo(HaveOccurred())
		again, err := NewService(client).KOSPIMarketCapSummary(context.Background(), "20260515")
		Expect(err).NotTo(HaveOccurred())

		Expect(summary.TotalMarketCap).To(BeNumerically("~", 400.0, 1e-9))
		Expect(again.TotalMarketCap).To(Equal(summary.TotalMarketCap))
		Expect(summary.Constituents).To(HaveLen(2))
		Expect(summary.Constituents[0].Code).To(Equal("000002"))
		Expect(resolveKOSPIMasterJSONPath(resolveKOSPIMasterCachePath(baseCachePath, "20260515"))).To(BeAnExistingFile())
		Expect(transport.Requests()).To(HaveLen(1))
		Expect(transport.Verify()).To(Succeed())
	})
})
