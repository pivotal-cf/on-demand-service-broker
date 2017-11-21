package herottp_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHerottp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HeroTTP Suite")
}
