package domesticstock

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type KOSPIMarketCapConstituent struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	MarketCap float64 `json:"market_cap"`
	BaseDate  string  `json:"base_date"`
}

type KOSPIMarketCapSummary struct {
	BusinessDate   string                      `json:"business_date"`
	TotalMarketCap float64                     `json:"total_market_cap"`
	Constituents   []KOSPIMarketCapConstituent `json:"constituents"`
}

func (s *Service) KOSPIMarketCapSummary(ctx context.Context, businessDate string) (*KOSPIMarketCapSummary, error) {
	businessDate = strings.TrimSpace(businessDate)
	if businessDate == "" {
		return nil, fmt.Errorf("businessDate is required")
	}

	records, err := s.loadKOSPIMaster(ctx, businessDate)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("kospi master file did not contain usable records")
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].MarketCap > records[j].MarketCap
	})

	summary := &KOSPIMarketCapSummary{
		BusinessDate: businessDate,
		Constituents: make([]KOSPIMarketCapConstituent, 0, len(records)),
	}
	for _, record := range records {
		summary.TotalMarketCap += record.MarketCap
		summary.Constituents = append(summary.Constituents, KOSPIMarketCapConstituent{
			Code:      record.Code,
			Name:      record.Name,
			MarketCap: record.MarketCap,
			BaseDate:  record.BaseDate,
		})
	}
	return summary, nil
}
