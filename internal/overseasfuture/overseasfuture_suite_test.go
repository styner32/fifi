package overseasfuture

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOverseasFuture(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Overseas Future Suite")
}
