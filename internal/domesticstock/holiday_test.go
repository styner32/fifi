package domesticstock

import (
	"context"
	"net/http"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KIS daily holiday cache", func() {
	var svc *Service
	var transport *testhelpers.MockTransport
	var now time.Time
	var dir string
	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		now = time.Date(2026, 10, 16, 10, 0, 0, 0, kst.Location)
		transport = testhelpers.NewMockTransport()
		client := auth.NewKIClient("key", "secret", "https://example.test", "test")
		client.Client = &http.Client{Transport: transport}
		client.SetAuthToken("test-token")
		svc = NewService(client)
	})
	Context("retrieval-day request budget", func() {
		It("reuses the daily response across target dates and validates opnd_yn", func() {
			transport.New("https://example.test").Get(holidayPath+"?BASS_DT=20261016&CTX_AREA_FK=&CTX_AREA_NK=").MatchHeader("tr_id", "CTCA0903R").Reply(http.StatusOK).JSON(map[string]any{"rt_cd": "0", "output": []map[string]any{{"bass_dt": "20261016", "opnd_yn": "Y"}, {"bass_dt": "20261019", "opnd_yn": "N"}}})
			open, err := svc.tradingDay(context.Background(), "20261016", now, dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(open.Open).To(BeTrue())
			closed, err := svc.tradingDay(context.Background(), "20261019", now.Add(time.Minute), dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(closed.Open).To(BeFalse())
			Expect(closed.CheckedAt).To(BeTemporally("==", now))
			_, err = svc.tradingDay(context.Background(), "20261020", now, dir)
			Expect(err).To(HaveOccurred())
			Expect(transport.Requests()).To(HaveLen(1))
			Expect(transport.Verify()).To(Succeed())
		})
		It("caches business failures to avoid retrying repeatedly within one day", func() {
			transport.New("https://example.test").Get(holidayPath + "?BASS_DT=20261016&CTX_AREA_FK=&CTX_AREA_NK=").Reply(http.StatusOK).JSON(map[string]any{"rt_cd": "1"})
			_, err := svc.tradingDay(context.Background(), "20261016", now, dir)
			Expect(err).To(HaveOccurred())
			_, err = svc.tradingDay(context.Background(), "20261016", now, dir)
			Expect(err).To(HaveOccurred())
			Expect(transport.Requests()).To(HaveLen(1))
		})
	})
})
