package cf_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestClientSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client System Test Suite")
}
