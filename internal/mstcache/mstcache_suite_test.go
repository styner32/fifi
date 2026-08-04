package mstcache

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMstcache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mstcache Suite")
}
