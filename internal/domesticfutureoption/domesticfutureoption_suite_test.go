package domesticfutureoption

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDomesticFutureOption(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DomesticFutureOption Suite")
}
