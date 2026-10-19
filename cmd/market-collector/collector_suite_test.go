package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestMarketCollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Market Collector Suite")
}
