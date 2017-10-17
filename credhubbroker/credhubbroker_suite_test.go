package credhubbroker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCredhubbroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credhubbroker Suite")
}
