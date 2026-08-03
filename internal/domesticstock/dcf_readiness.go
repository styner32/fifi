package domesticstock

import (
	"context"
	"errors"
	"strings"
)

func (s *Service) DCFReadiness(ctx context.Context, symbol string, divClsCode string) (*DCFReadinessResult, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	bundle, err := s.buildDCFInputBundle(ctx, symbol, DCFValuationOptions{DivClsCode: divClsCode})
	if err != nil {
		return nil, err
	}

	result := &DCFReadinessResult{
		Symbol:         bundle.Symbol,
		Division:       bundle.Division,
		BalancePeriods: len(bundle.BalanceRows),
		IncomePeriods:  len(bundle.IncomeRows),
		Inputs:         bundle.Inputs,
	}
	result.MissingForFCF = missingInputNames(result.Inputs, "Revenue", "EBIT", "EffectiveTax", "DnA", "CapEx", "ChangeInNWC")
	result.MissingForWACC = missingInputNames(result.Inputs, "RiskFreeRate", "Beta", "MarketPremium", "CostOfDebt", "EquityWeight", "DebtWeight", "EffectiveTax")
	result.MissingForTargetPrice = missingInputNames(result.Inputs, "SharesOut", "NetDebt")
	result.CanProjectFCF = len(result.MissingForFCF) == 0
	result.CanComputeWACC = len(result.MissingForWACC) == 0
	result.CanComputeEnterpriseValue = result.CanProjectFCF && result.CanComputeWACC
	result.CanComputeTargetPrice = result.CanComputeEnterpriseValue && len(result.MissingForTargetPrice) == 0
	return result, nil
}
