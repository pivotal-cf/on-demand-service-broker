package cf_test

import (
	"io"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
)

var _ = Describe("/v2/service_plans contract", func() {
	var (
		subject *cf.Client
		logger  *log.Logger
	)

	BeforeEach(func() {
		logBuffer := gbytes.NewBuffer()
		logger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
		subject = NewCFClient(logger)
	})

	It("works for GetServiceInstances()", func() {
		instances, err := subject.GetServiceInstances(cf.GetInstancesFilter{
			ServiceOfferingID: brokerDeployment.ServiceID,
		}, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(instances).To(HaveLen(1))
		Expect(instances[0].PlanUniqueID).To(Equal(brokerDeployment.PlanID + "-small"))
		Expect(instances[0].GUID).To(Equal(serviceInstanceGUID))
	})
})
