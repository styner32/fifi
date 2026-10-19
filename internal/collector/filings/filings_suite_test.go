package filings_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFilings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filings Collector Suite")
}

