package contract_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestContractTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Contract Tests Suite")
}
