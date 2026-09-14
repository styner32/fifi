package domesticbond

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("DomesticBond Service", func() {
	Context("InquirePrice", func() {
		It("fetches bond price details successfully", func() {
			transport := testhelpers.NewMockTransport()
			transport.New("https://example.test").
				Get("/uapi/domestic-bond/v1/quotations/inquire-price?FID_COND_MRKT_DIV_CODE=B&FID_INPUT_ISCD=KR6000291999").
				Reply(http.StatusOK).
				Header("content-type", "application/json").
				BodyString(`{
					"rt_cd": "0",
					"msg_cd": "SUCCESS",
					"msg1": "Success",
					"output": {
						"bond_prpr": "10050.25",
						"bond_name": "국고0300-2909"
					}
				}`)

			client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
			client.Client = &http.Client{Transport: transport}
			client.AuthToken = "dummy-token"

			svc := NewService(client)
			resp, err := svc.InquirePrice(context.Background(), "B", "KR6000291999")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.IsOK()).To(BeTrue())
			Expect(resp.MessageCode()).To(Equal("SUCCESS"))
			Expect(transport.Verify()).To(Succeed())
		})

		It("returns error when inputISCD is empty", func() {
			client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
			svc := NewService(client)

			_, err := svc.InquirePrice(context.Background(), "B", "")
			Expect(err).To(HaveOccurred())
		})
	})
})
