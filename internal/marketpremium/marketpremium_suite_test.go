package marketpremium

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMarketPremium(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MarketPremium Suite")
}
