package shippingindex

import (
	"context"
	"net/http"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Service", func() {
	It("parses requested shipping indices from the StockQ homepage summary table", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://www.indexq.org").
			Get("/").
			Reply(http.StatusOK).
			BodyString(`
<html><body>
<tr class='row2'>
<td align='left' nowrap><a href="/index/SCFI.php">SCFI</a></td>
<td nowrap>1710.35</td>
<td nowrap class="changeup">221.16</td>
<td nowrap class="changeup">14.85%</td>
<td nowrap align=center>03/13</td>
</tr>
<tr class='row1'>
<td align='left' nowrap><a href="/index/BDI.php">Baltic Dry</a></td>
<td nowrap>2028.00</td>
<td nowrap class="changeup">56.00</td>
<td nowrap class="changeup">2.84%</td>
<td nowrap align=center>03/13</td>
</tr>
<tr class='row2'>
<td align='left' nowrap><a href="/index/BCTI.php">Baltic Clean Tanker</a></td>
<td nowrap>1463.00</td>
<td nowrap class="changedown">-8.00</td>
<td nowrap class="changedown">-0.54%</td>
<td nowrap align=center>03/13</td>
</tr>
</body></html>`)

		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		quotes, err := svc.Quotes(context.Background(), []string{"SCFI", "BDI", "BCTI"})
		Expect(err).NotTo(HaveOccurred())
		Expect(quotes).To(HaveLen(3))

		Expect(quotes[0].Symbol).To(Equal("SCFI"))
		Expect(quotes[0].Price).To(BeNumerically("==", 1710.35))
		Expect(quotes[0].ChangePercent).To(BeNumerically("==", 14.85))

		Expect(quotes[1].Symbol).To(Equal("BDI"))
		Expect(quotes[1].Name).To(Equal("Baltic Dry"))
		Expect(quotes[1].Change).To(BeNumerically("==", 56))

		Expect(quotes[2].Symbol).To(Equal("BCTI"))
		Expect(quotes[2].Change).To(BeNumerically("==", -8))
	})

	It("returns a descriptive error when a requested symbol is missing", func() {
		quotes, err := parseHomepage([]byte(`
<tr class='row1'>
<td align='left' nowrap><a href="/index/BDI.php">Baltic Dry</a></td>
<td nowrap>2028.00</td>
<td nowrap class="changeup">56.00</td>
<td nowrap class="changeup">2.84%</td>
<td nowrap align=center>03/13</td>
</tr>`))
		Expect(err).NotTo(HaveOccurred())
		Expect(quotes).To(HaveKey("BDI"))

		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})
		transport.New("https://www.indexq.org").
			Get("/").
			Reply(http.StatusOK).
			BodyString(`
<tr class='row1'>
<td align='left' nowrap><a href="/index/BDI.php">Baltic Dry</a></td>
<td nowrap>2028.00</td>
<td nowrap class="changeup">56.00</td>
<td nowrap class="changeup">2.84%</td>
<td nowrap align=center>03/13</td>
</tr>`)
		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		result, svcErr := svc.Quotes(context.Background(), []string{"BDI", "SCFI"})
		Expect(result).To(HaveLen(1))
		Expect(svcErr).To(MatchError(ContainSubstring("SCFI")))
	})
})
