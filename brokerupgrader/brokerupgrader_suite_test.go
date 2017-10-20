package brokerupgrader_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBrokerupgrader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Brokerupgrader Suite")
}
