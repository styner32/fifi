package envcfg

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEnvcfg(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Envcfg Suite")
}
