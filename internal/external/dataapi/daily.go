package dataapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type DailyStockRow struct {
	Date   string `json:"basDt"`
	Code   string `json:"srtnCd"`
	ISIN   string `json:"isinCd"`
	Market string `json:"mrktCtg"`
	Change string `json:"vs"`
	Close  string `json:"clpr"`
	Volume string `json:"trqu"`
}

// DailyStockPrices fetches the complete published cross-section for one date.
// No fallback to an older date: delayed publication must remain explicit.
func (c *DataAPIClient) DailyStockPrices(ctx context.Context, date string) ([]DailyStockRow, error) {
	if _, err := time.Parse("20060102", date); err != nil {
		return nil, fmt.Errorf("invalid business date")
	}
	if c.key == "" {
		return nil, fmt.Errorf("DATA_API_KEY not configured")
	}
	out := []DailyStockRow{}
	expected := -1
	for page := 1; page <= 20; page++ {
		q := url.Values{"serviceKey": {c.key}, "numOfRows": {"1000"}, "pageNo": {strconv.Itoa(page)}, "resultType": {"json"}, "basDt": {date}}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"1160100/service/GetStockSecuritiesInfoService/getStockPriceInfo?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("cannot create daily price request")
		}
		resp, err := c.client.Do(req)
		// net/http errors include the URL, which contains the service key.
		if err != nil {
			return nil, fmt.Errorf("daily price request failed (network or timeout)")
		}
		var decoded struct {
			Response struct {
				Header struct {
					Code string `json:"resultCode"`
				} `json:"header"`
				Body struct {
					Total int `json:"totalCount"`
					Page  int `json:"pageNo"`
					Items struct {
						Rows []DailyStockRow `json:"item"`
					} `json:"items"`
				} `json:"body"`
			} `json:"response"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&decoded)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || err != nil {
			return nil, fmt.Errorf("daily price HTTP %d or invalid JSON", resp.StatusCode)
		}
		r := decoded.Response
		if r.Header.Code != "00" {
			return nil, fmt.Errorf("daily price business error %s", r.Header.Code)
		}
		if r.Body.Total <= 0 {
			return nil, fmt.Errorf("daily prices not published for %s", date)
		}
		if expected < 0 {
			expected = r.Body.Total
		}
		if r.Body.Total != expected || r.Body.Page != page || len(r.Body.Items.Rows) == 0 {
			return nil, fmt.Errorf("incomplete or changing daily price pagination")
		}
		out = append(out, r.Body.Items.Rows...)
		if len(out) == expected {
			return out, nil
		}
		if len(out) > expected {
			return nil, fmt.Errorf("daily price count exceeds total")
		}
	}
	return nil, fmt.Errorf("daily price pagination limit exceeded")
}
