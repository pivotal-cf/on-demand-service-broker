package recreate_all_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRecreateAll(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RecreateAll Suite")
}
