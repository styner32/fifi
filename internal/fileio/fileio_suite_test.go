package fileio

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFileio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fileio Suite")
}
