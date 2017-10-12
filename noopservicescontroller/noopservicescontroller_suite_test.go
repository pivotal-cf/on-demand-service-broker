package noopservicescontroller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDummy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NoopCF Client Suite")
}
