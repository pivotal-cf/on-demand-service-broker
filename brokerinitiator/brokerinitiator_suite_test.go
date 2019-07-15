package brokerinitiator_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBrokerinitiator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Brokerinitiator Suite")
}
