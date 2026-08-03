package pulse

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPulse(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pulse Suite")
}
