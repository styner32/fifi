package kofia

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKofia(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kofia Suite")
}
