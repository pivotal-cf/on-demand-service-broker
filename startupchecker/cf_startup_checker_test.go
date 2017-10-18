package startupchecker_test

import (
	"github.com/pivotal-cf/on-demand-service-broker/config"
	. "github.com/pivotal-cf/on-demand-service-broker/startupchecker"

	"errors"

	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker/fakes"
)

var _ = Describe("CFStartupChecker", func() {
	const (
		minimumCFVersion string = "2.57.0"
		oldCFVersion     string = "2.56.0"
		invalidCFVersion string = "1.invalid.0"
		existingPlanID   string = "existing-plan"
	)

	var (
		client         *fakes.FakeCloudFoundryClient
		serviceCatalog = config.ServiceOffering{
			ID: "service-id",
			Plans: []config.Plan{
				{ID: existingPlanID},
			},
		}
		noLogTesting *log.Logger
	)

	BeforeEach(func() {
		client = new(fakes.FakeCloudFoundryClient)
	})

	It("exhibits success when CF API is current", func() {
		client.GetAPIVersionReturns(minimumCFVersion, nil)
		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("exhibits success when CF API is the next major version", func() {
		client.GetAPIVersionReturns("3.0.0", nil)
		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("produces error when CF API is out of date", func() {
		client.GetAPIVersionReturns(oldCFVersion, nil)
		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError("CF API error: Cloud Foundry API version is insufficient, ODB requires CF v238+."))
	})

	It("produces error when CF API responds with error", func() {
		cfAPIFailureMessage := "Failed to contact CF API"
		client.GetAPIVersionReturns("", errors.New(cfAPIFailureMessage))
		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError("CF API error: " + cfAPIFailureMessage + ". ODB requires CF v238+."))
	})

	It("produces error if the CF API version cannot be parsed", func() {
		client.GetAPIVersionReturns(invalidCFVersion, nil)
		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError("Cloud Foundry API version couldn't be parsed. Expected a semver, got: 1.invalid.0."))
	})

	It("exhibits success when no service instances deployed", func() {
		client.GetAPIVersionReturns(minimumCFVersion, nil)
		client.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{}, nil)
		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error if service catalog does not contain the existing instance's plan", func() {
		client.GetAPIVersionReturns(minimumCFVersion, nil)
		client.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
			cfServicePlan("non-existent-plan-id", "non-existent-plan"): 1,
		}, nil)

		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError(
			"plan non-existent-plan (non-existent-plan-id) was expected but is now missing. You cannot remove or change the plan_id of a plan which has existing service instances",
		))
	})

	It("returns no error when there are no pre-existing instances of configured plans and service catalog contains the existing instance's plan", func() {
		client.GetAPIVersionReturns(minimumCFVersion, nil)
		client.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
			cfServicePlan(existingPlanID, "existing-plan"): 1,
		}, nil)
		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns no error when the service catalog does not contain a plan with zero instances", func() {
		client.GetAPIVersionReturns(minimumCFVersion, nil)
		client.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
			cfServicePlan(existingPlanID, "existing-plan"):             1,
			cfServicePlan("non-existent-plan-id", "non-existent-plan"): 0,
		}, nil)
		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when instances cannot be retrieved", func() {
		client.GetAPIVersionReturns(minimumCFVersion, nil)
		client.CountInstancesOfServiceOfferingReturns(nil, errors.New("error counting instances"))

		c := NewCFChecker(client, minimumCFVersion, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).To(HaveOccurred())
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
