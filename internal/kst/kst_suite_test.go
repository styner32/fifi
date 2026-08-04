package kst

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKst(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kst Suite")
}
