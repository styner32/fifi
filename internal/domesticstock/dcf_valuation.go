package domesticstock

import (
	"context"

	"github.com/fifi/internal/dcf"
)

func (s *Service) DCFValuation(ctx context.Context, symbol string, options DCFValuationOptions) (*DCFValuationResult, error) {
	bundle, err := s.buildDCFInputBundle(ctx, symbol, options)
	if err != nil {
		return nil, err
	}

	valuation, err := dcf.Value(bundle.Financial, bundle.Market, bundle.Assumptions, bundle.Projection)
	if err != nil {
		return nil, err
	}

	return &DCFValuationResult{
		Symbol:       bundle.Symbol,
		Division:     bundle.Division,
		CurrentPrice: bundle.CurrentPrice,
		Inputs:       bundle.Inputs,
		Notes:        bundle.Notes,
		Financial:    bundle.Financial,
		Market:       bundle.Market,
		Assumptions:  bundle.Assumptions,
		Projection:   bundle.Projection,
		Valuation:    valuation,
	}, nil
}
