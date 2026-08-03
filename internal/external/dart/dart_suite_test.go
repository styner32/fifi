package dart_test

import (
	"testing"

	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDart(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dart Suite")
}

var _ = BeforeSuite(func() {
	testhelpers.LoadTestEnv()
})
