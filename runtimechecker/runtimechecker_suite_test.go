package runtimechecker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRuntimechecker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runtimechecker Suite")
}
