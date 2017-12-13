package startupchecker_test

import (
	. "github.com/pivotal-cf/on-demand-service-broker/startupchecker"

	"errors"

	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker/fakes"
)

var _ = Describe("CFAPIVersionChecker", func() {
	const (
		minimumCFVersion = "2.57.0"
		oldCFVersion     = "2.56.0"
		invalidCFVersion = "1.invalid.0"
	)

	var (
		client       *fakes.FakeCFAPIVersionGetter
		noLogTesting *log.Logger
	)

	BeforeEach(func() {
		client = new(fakes.FakeCFAPIVersionGetter)
	})

	It("exhibits success when CF API is current", func() {
		client.GetAPIVersionReturns(minimumCFVersion, nil)
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("exhibits success when CF API is the next major version", func() {
		client.GetAPIVersionReturns("3.0.0", nil)
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("produces error when CF API is out of date", func() {
		client.GetAPIVersionReturns(oldCFVersion, nil)
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError("CF API error: Cloud Foundry API version is insufficient, ODB requires CF v238+."))
	})

	It("produces error when CF API responds with error", func() {
		cfAPIFailureMessage := "Failed to contact CF API"
		client.GetAPIVersionReturns("", errors.New(cfAPIFailureMessage))
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError("CF API error: " + cfAPIFailureMessage + ". ODB requires CF v238+."))
	})

	It("produces error if the CF API version cannot be parsed", func() {
		client.GetAPIVersionReturns(invalidCFVersion, nil)
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError("Cloud Foundry API version couldn't be parsed. Expected a semver, got: 1.invalid.0."))
	})
})

func cfServicePlan(uniqueID, name string) cf.ServicePlan {
	return cf.ServicePlan{
		ServicePlanEntity: cf.ServicePlanEntity{
			UniqueID: uniqueID,
			Name:     name,
		},
	}
}
