package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("KIClient GET", func() {
	It("refreshes an expired token and retries the request", func() {
		client := NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		tokenCachePath := filepath.Join(GinkgoT().TempDir(), ".auth_token.json")
		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/test").
			MatchHeader("Authorization", "Bearer expired-token").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd":  "1",
				"msg_cd": "EGW00123",
				"msg1":   "기간이 만료된 token 입니다.",
			})

		transport.New("https://example.test").
			Post("/oauth2/tokenP").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"access_token":               "refreshed-token",
				"token_type":                 "Bearer",
				"expires_in":                 3600,
				"access_token_token_expired": "2099-01-01 00:00:00",
			})

		transport.New("https://example.test").
			Get("/uapi/test").
			MatchHeader("Authorization", "Bearer refreshed-token").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "00000",
				"msg1":   "정상처리 되었습니다.",
				"output": map[string]any{
					"value": "ok",
				},
			})

		client.Client = &http.Client{
			Transport: transport,
		}
		client.SetTokenCachePath(tokenCachePath)
		client.SetAuthToken("expired-token")

		resp, err := client.Get(context.Background(), "/uapi/test", "FTEST000000", "", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
		Expect(resp.IsOK()).To(BeTrue(), "msg_cd=%s msg1=%s", resp.MessageCode(), resp.Message())
		Expect(client.AuthToken).To(Equal("refreshed-token"))

		rawCache, err := os.ReadFile(tokenCachePath)
		Expect(err).NotTo(HaveOccurred())

		var cachedToken CachedToken
		Expect(json.Unmarshal(rawCache, &cachedToken)).To(Succeed())
		Expect(cachedToken.AccessToken).To(Equal("refreshed-token"))

		requests := transport.Requests()
		Expect(requests).To(HaveLen(3))
		Expect(requests[0].Method).To(Equal(http.MethodGet))
		Expect(requests[0].URL).To(Equal("https://example.test/uapi/test"))
		Expect(requests[0].Headers.Get("Authorization")).To(Equal("Bearer expired-token"))
		Expect(requests[1].Method).To(Equal(http.MethodPost))
		Expect(requests[1].URL).To(Equal("https://example.test/oauth2/tokenP"))
		Expect(requests[2].Method).To(Equal(http.MethodGet))
		Expect(requests[2].URL).To(Equal("https://example.test/uapi/test"))
		Expect(requests[2].Headers.Get("Authorization")).To(Equal("Bearer refreshed-token"))

		metrics := client.MetricsSnapshot()
		Expect(metrics.CallCount).To(Equal(3))
		Expect(metrics.SuccessCount).To(Equal(3))
		Expect(metrics.ErrorCount).To(Equal(0))
		Expect(metrics.TotalDuration).To(BeNumerically(">=", 0))
		Expect(metrics.RPM).To(BeNumerically(">=", 0))
	})

	It("returns raw body when JSON parsing fails", func() {
		client := NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/bad-json").
			MatchHeader("Authorization", "Bearer test-token").
			Reply(http.StatusOK).
			BodyString("{\"rt_cd\":\"0\",\"msg1\":\"partial\"")

		client.Client = &http.Client{Transport: transport}

		resp, err := client.Get(context.Background(), "/uapi/bad-json", "FTESTBADJSON", "", nil)
		Expect(err).To(HaveOccurred())
		Expect(resp).NotTo(BeNil())
		Expect(resp.ParseError).NotTo(BeEmpty())
		Expect(string(resp.RawBody)).To(ContainSubstring("\"msg1\":\"partial\""))
	})

	It("returns an explicit empty-body parse error when the response body is empty", func() {
		client := NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/empty").
			MatchHeader("Authorization", "Bearer test-token").
			Reply(http.StatusOK)

		client.Client = &http.Client{Transport: transport}

		resp, err := client.Get(context.Background(), "/uapi/empty", "FTESTEMPTY", "", nil)
		Expect(err).To(HaveOccurred())
		Expect(resp).NotTo(BeNil())
		Expect(resp.ParseError).To(Equal("empty response body"))
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.RequestMethod).To(Equal(http.MethodGet))
		Expect(resp.RequestURL).To(Equal("https://example.test/uapi/empty"))
		Expect(resp.RawBody).To(BeEmpty())
	})
})
