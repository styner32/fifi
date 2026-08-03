package domesticstock

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDomesticStock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DomesticStock Suite")
}
