package naver

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNaver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Naver Suite")
}
