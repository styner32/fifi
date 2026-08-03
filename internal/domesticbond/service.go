package domesticbond

import (
	"context"
	"errors"
	"strings"

	"github.com/fifi/internal/auth"
)

const (
	inquirePricePath         = "/uapi/domestic-bond/v1/quotations/inquire-price"
	inquirePriceTRID         = "FHKBJ773400C0"
	defaultBondMarketDivCode = "B"
)

type Service struct {
	client *auth.KIClient
}

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}

func (s *Service) InquirePrice(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = defaultBondMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_INPUT_ISCD":         inputISCD,
	}
	return s.client.Get(ctx, inquirePricePath, inquirePriceTRID, "", params)
}
