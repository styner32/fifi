package commodityfuture

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCommodityFuture(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CommodityFuture Suite")
}
