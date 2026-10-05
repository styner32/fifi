package domesticstock

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type KOSPIMarketCapConstituent struct {
	KOSPIIndexMember bool    `json:"kospi_index_member"`
	PreferredClass   string  `json:"preferred_class"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	MarketCap        float64 `json:"market_cap"`
	BaseDate         string  `json:"base_date"`
}

type KOSPIMarketCapSummary struct {
	Universe       string                      `json:"universe"`
	WeightStatus   string                      `json:"weight_status"`
	SourcePath     string                      `json:"source_path"`
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
		BusinessDate: businessDate, Universe: "KIS_MARKET_INCLUDING_PREFERRED", WeightStatus: "NOT_INDEX_WEIGHTS",
		Constituents: make([]KOSPIMarketCapConstituent, 0, len(records)),
	}
	for _, record := range records {
		summary.TotalMarketCap += record.MarketCap
		summary.Constituents = append(summary.Constituents, KOSPIMarketCapConstituent{
			KOSPIIndexMember: record.KOSPIIndexMember, PreferredClass: record.PreferredClass,
			Code:      record.Code,
			Name:      record.Name,
			MarketCap: record.MarketCap,
			BaseDate:  record.BaseDate,
		})
	}
	return summary, nil
}
