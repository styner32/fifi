package overseasstock

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOverseasStock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Overseas Stock Suite")
}
