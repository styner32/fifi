package fifimcp

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFifiMCP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Unified Fifi MCP Suite")
}
