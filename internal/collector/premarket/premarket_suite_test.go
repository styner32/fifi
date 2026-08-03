package premarket_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPremarket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Premarket Suite")
}
