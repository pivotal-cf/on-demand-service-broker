package noopservicescontroller_test

import (
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"

	"io"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"

	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Client", func() {

	var testLogger *log.Logger

	BeforeEach(func() {
		logBuffer := gbytes.NewBuffer()
		testLogger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
	})

	Describe("New", func() {
		It("should create client", func() {
			Expect(noopservicescontroller.New()).ToNot(BeNil())
		})

		It("should be CloudFoundry", func() {
			client := noopservicescontroller.New()
			var i interface{} = client
			_, ok := i.(broker.CloudFoundryClient)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("GetAPIVersion", func() {
		It("return a valid version", func() {
			client := noopservicescontroller.New()
			version, err := client.GetAPIVersion(testLogger)
			Expect(err).To(BeNil())
			Expect(version).To(Equal(broker.MinimumCFVersion))
		})
	})

	Describe("CountInstancesOfPlan", func() {
		It("always returns 1", func() {
			client := noopservicescontroller.New()
			planCount, err := client.CountInstancesOfPlan("offeringId", "planId", testLogger)
			Expect(err).To(BeNil())
			Expect(planCount).To(Equal(1))
		})
	})

	Describe("CountInstancesOfServiceOffering", func() {
		It("returns empty map", func() {
			client := noopservicescontroller.New()
			instanceCountByPlanID, err := client.CountInstancesOfServiceOffering("offeringId", testLogger)
			Expect(err).To(BeNil())
			Expect(instanceCountByPlanID).ToNot(BeNil())
		})
	})

	Describe("GetInstanceState", func() {
		It("return default state", func() {
			client := noopservicescontroller.New()
			instanceState, err := client.GetInstanceState("serviceInstanceGUID", testLogger)
			Expect(err).To(BeNil())
			Expect(instanceState).ToNot(BeNil())
		})
	})

	Describe("GetInstancesOfServiceOffering", func() {
		It("gets empty instances of service offerings", func() {
			client := noopservicescontroller.New()
			instances, err := client.GetInstancesOfServiceOffering("serviceInstanceGUID", testLogger)
			Expect(err).To(BeNil())
			Expect(instances).ToNot(BeNil())
		})
	})

})
