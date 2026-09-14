package domesticbond

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDomesticbond(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Domesticbond Suite")
}
