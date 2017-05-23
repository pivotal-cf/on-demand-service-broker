package registrar_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRegistrar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registrar Suite")
}
