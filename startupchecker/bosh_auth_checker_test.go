package startupchecker_test

import (
	. "github.com/pivotal-cf/on-demand-service-broker/startupchecker"

	"log"

	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker/fakes"
)

var _ = Describe("BOSH Auth Checker", func() {
	var (
		untestedLogger   *log.Logger
		fakeAuthVerifier *fakes.FakeAuthVerifier
	)

	BeforeEach(func() {
		fakeAuthVerifier = new(fakes.FakeAuthVerifier)
	})

	It("returns no error when auth succeeds", func() {
		fakeAuthVerifier.VerifyAuthReturns(nil)
		c := NewBOSHAuthChecker(fakeAuthVerifier, untestedLogger)
		err := c.Check()

		Expect(err).NotTo(HaveOccurred())
	})

	It("produces error when auth fails", func() {
		fakeAuthVerifier.VerifyAuthReturns(errors.New("I love errors"))
		c := NewBOSHAuthChecker(fakeAuthVerifier, untestedLogger)
		err := c.Check()

		Expect(err).To(MatchError("BOSH Director error: I love errors"))
	})
})
