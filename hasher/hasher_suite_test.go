package hasher_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHasher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hasher Suite")
}
