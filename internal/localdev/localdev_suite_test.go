package localdev

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLocaldev(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Local Development Environment Suite")
}
