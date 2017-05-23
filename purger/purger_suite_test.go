package purger_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPurger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Purger Suite")
}
