package credstore_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCredStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CredStore Suite")
}
