package marketpremium

import (
	"context"
	"net/http"
	"os"

	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolver", func() {
	It("parses the current Damodaran implied ERP", func() {
		transport := testhelpers.NewMockTransport()
		transport.New("https://pages.stern.nyu.edu").
			Get("/adamodar/New_Home_Page/home.htm").
			Reply(http.StatusOK).
			BodyString(`<html><body>Implied ERP on February 1, 2026 = 4.17% (Trailing 12 month, with adjusted payout); Est. year-end ERP = 4.40%</body></html>`)

		resolver := Resolver{HTTPClient: &http.Client{Transport: transport}}
		value, err := resolver.Resolve(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(value.Provider).To(Equal("damodaran"))
		Expect(value.Rate).To(BeNumerically("~", 0.0417, 1e-9))
		Expect(value.AsOf).To(Equal("February 1, 2026"))
		Expect(transport.Verify()).To(Succeed())
	})

	It("loads a file-based market premium when configured", func() {
		tmpDir := GinkgoT().TempDir()
		path := tmpDir + "/market_premium.json"
		Expect(os.WriteFile(path, []byte(`{"market_premium":4.25,"as_of":"2026-03-10","note":"manual file"}`), 0o600)).To(Succeed())

		Expect(os.Setenv(ProviderEnvKey, "file")).To(Succeed())
		Expect(os.Setenv(FileEnvKey, path)).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(ProviderEnvKey)).To(Succeed())
			Expect(os.Unsetenv(FileEnvKey)).To(Succeed())
		})

		value, err := Resolver{}.Resolve(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(value.Provider).To(Equal("file"))
		Expect(value.Rate).To(BeNumerically("~", 0.0425, 1e-9))
		Expect(value.AsOf).To(Equal("2026-03-10"))
	})
})
