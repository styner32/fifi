package quadwitching

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestQuadWitching(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QuadWitching Suite")
}
