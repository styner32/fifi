package shippingindex

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShippingIndex(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shipping Index Suite")
}
