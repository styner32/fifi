package dcf

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDCF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DCF Suite")
}
