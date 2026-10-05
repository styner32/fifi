package domesticstock

import (
	"context"
	"github.com/fifi/internal/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

type premarketTestTransport func(http.ResponseWriter, *http.Request)

func (f premarketTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	f(w, req)
	return w.Result(), nil
}
func TestPremarketKOSDAQInvestorRequest(t *testing.T) {
	handler := premarketTestTransport(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("FID_INPUT_ISCD") != "1001" || q.Get("FID_INPUT_ISCD_1") != "KSQ" || q.Get("FID_INPUT_ISCD_2") != "1001" || q.Get("FID_INPUT_DATE_1") != "20260911" || q.Get("FID_INPUT_DATE_2") != "20260911" {
			t.Errorf("wrong KOSDAQ parameters: %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rt_cd":"0","output":[]}`))
	})
	c := auth.NewKIClient("key", "secret", "https://example.test", "test")
	c.SetAuthToken("token")
	c.Client = &http.Client{Transport: handler}
	s := NewService(c)
	if _, err := s.InquireInvestorDailyForMarket(context.Background(), "KOSDAQ", "20260911"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InquireInvestorDailyForMarket(context.Background(), "INVALID", "20260911"); err == nil {
		t.Fatal("unknown market accepted")
	}
}
