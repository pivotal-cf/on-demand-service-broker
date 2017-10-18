package startupchecker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStartupchecker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Startupchecker Suite")
}
