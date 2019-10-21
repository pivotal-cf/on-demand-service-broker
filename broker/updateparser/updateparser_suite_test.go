package updateparser_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUpdateparser(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Updateparser Suite")
}
