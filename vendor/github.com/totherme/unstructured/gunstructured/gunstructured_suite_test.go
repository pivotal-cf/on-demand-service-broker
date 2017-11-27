package gunstructured_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMatchers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gunstructured Matchers Suite")
}
