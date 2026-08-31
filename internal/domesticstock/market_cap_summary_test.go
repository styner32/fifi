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
	"golang.org/x/text/encoding/korean"
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

	It("우선주(preferred stocks)를 포함하여 시총 순으로 정렬", func() {
		cacheDir := GinkgoT().TempDir()
		baseCachePath := filepath.Join(cacheDir, "kospi_code.mst")
		Expect(os.Setenv(kospiMasterCacheEnvKey, baseCachePath)).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(kospiMasterCacheEnvKey)).To(Succeed())
		})

		masterBody := strings.Join([]string{
			buildKOSPIMasterLineFull("A005930", "삼성전자", "ST", "0", "Y", "1000", "20", "20250331", "50000"),
			buildKOSPIMasterLineFull("A005935", "삼성전자우", "ST", "1", "N", "0", "0", "20250331", "10000"),
			buildKOSPIMasterLineFull("A000660", "SK하이닉스", "ST", "0", "Y", "500", "15", "20250331", "30000"),
			buildKOSPIMasterLineFull("A123456", "KODEX200", "EF", "0", "N", "0", "0", "20250331", "20000"), // ETF는 제외
		}, "")
		euckrBody, err := korean.EUCKR.NewEncoder().Bytes([]byte(masterBody))
		Expect(err).NotTo(HaveOccurred())
		zipBody, err := testhelpers.CreateMockZipArchive(kospiMasterFilename, euckrBody)
		Expect(err).NotTo(HaveOccurred())

		transport := testhelpers.NewMockTransport()
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/kospi_code.mst.zip").
			Reply(http.StatusOK).
			Body(zipBody)

		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.Client = &http.Client{Transport: transport}
		summary, err := NewService(client).KOSPIMarketCapSummary(context.Background(), "20260831")
		Expect(err).NotTo(HaveOccurred())

		// ETF(123456)는 제외되고 보통주 2개 + 우선주 1개 총 3종목 포함
		Expect(summary.Constituents).To(HaveLen(3))
		Expect(summary.TotalMarketCap).To(BeNumerically("~", 90000.0, 1e-9))
		// 시총 순위: 삼성전자(50000) -> SK하이닉스(30000) -> 삼성전자우(10000)
		Expect(summary.Constituents[0].Code).To(Equal("005930"))
		Expect(summary.Constituents[0].Name).To(Equal("삼성전자"))
		Expect(summary.Constituents[1].Code).To(Equal("000660"))
		Expect(summary.Constituents[1].Name).To(Equal("SK하이닉스"))
		Expect(summary.Constituents[2].Code).To(Equal("005935"))
		Expect(summary.Constituents[2].Name).To(Equal("삼성전자우"))
	})
})
