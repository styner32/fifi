package auth

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("KIClient EnsureAuthToken", func() {
	It("uses a valid cached token without making a network request", func() {
		client := NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetTokenCachePath(filepath.Join(GinkgoT().TempDir(), ".auth_token.json"))
		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})
		client.Client = &http.Client{
			Transport: transport,
		}

		err := client.saveTokenCache(&TokenResponse{
			AccessToken:  "cached-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			TokenExpired: time.Now().Add(2 * time.Hour).Format("2006-01-02 15:04:05"),
		})
		Expect(err).NotTo(HaveOccurred())

		token, err := client.EnsureAuthToken(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(token.AccessToken).To(Equal("cached-token"))
		Expect(client.AuthToken).To(Equal("cached-token"))
		Expect(transport.Requests()).To(BeEmpty())

		metrics := client.MetricsSnapshot()
		Expect(metrics.CallCount).To(Equal(0))
		Expect(metrics.SuccessCount).To(Equal(0))
		Expect(metrics.ErrorCount).To(Equal(0))
	})
})
