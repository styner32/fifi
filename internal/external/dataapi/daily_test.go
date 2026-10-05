package dataapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type dailyTransport func(*http.Request) (*http.Response, error)

func (f dailyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestDailyPricesCompletePaginationAndBusinessErrors(t *testing.T) {
	calls := 0
	c := New("secret-key")
	c.client = &http.Client{Transport: dailyTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		if q.Get("basDt") != "20260911" || q.Get("pageNo") != fmt.Sprint(calls) {
			t.Error("missing exact date/page")
		}
		body := fmt.Sprintf(`{"response":{"header":{"resultCode":"00"},"body":{"pageNo":%d,"totalCount":2,"items":{"item":[{"basDt":"20260911","isinCd":"KR%d"}]}}}}`, calls, calls)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	rows, err := c.DailyStockPrices(context.Background(), "20260911")
	if err != nil || len(rows) != 2 || calls != 2 {
		t.Fatalf("%v %v calls=%d", rows, err, calls)
	}
	for _, body := range []string{
		`{"response":{"header":{"resultCode":"30"}}}`,
		`{"response":{"header":{"resultCode":"00"},"body":{"pageNo":1,"totalCount":0}}}`,
		`{"response":{"header":{"resultCode":"00"},"body":{"pageNo":1,"totalCount":2,"items":{"item":[]}}}}`,
	} {
		c.client = &http.Client{Transport: dailyTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
		if _, err := c.DailyStockPrices(context.Background(), "20260911"); err == nil {
			t.Fatal("accepted unpublished/partial/business-error data")
		}
	}
}
func TestDailyPricesRedactsAuthenticationInNetworkErrors(t *testing.T) {
	c := New("secret-key")
	c.client = &http.Client{Transport: dailyTransport(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("secret-key") })}
	_, err := c.DailyStockPrices(context.Background(), "20260911")
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatal("leaked service key")
	}
}
