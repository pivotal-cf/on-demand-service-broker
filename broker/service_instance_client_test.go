package broker_test

import (
	"encoding/json"
	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	brokerfakes "github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"log"
)

var _ = Describe("ServiceInstanceClient", func() {
	var (
		instanceID     string
		instanceName   string
		fakeUAAClient  *brokerfakes.FakeUAAClient
		expectedClient map[string]string
		rawContext     json.RawMessage
		logger         *log.Logger
	)

	BeforeEach(func() {
		instanceID = "some-instance"
		instanceName = "some-instance-name"
		rawContext, _ = json.Marshal(map[string]interface{}{
			"instance_name": instanceName,
		})

		fakeUAAClient = new(brokerfakes.FakeUAAClient)
		b = createDefaultBroker()
		b.SetUAAClient(fakeUAAClient)

		logger = loggerFactory.NewWithRequestID()

		expectedClient = map[string]string{
			"client_secret": "some-secret",
			"client_id":     "some-id",
			"foo":           "bar",
		}
		fakeUAAClient.UpdateClientReturns(expectedClient, nil)
	})

	Describe("#GetServiceInstanceClient", func() {
		It("looks for the client on UAA", func() {
			_, err := b.GetServiceInstanceClient(instanceID, rawContext)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeUAAClient.GetClientCallCount()).To(Equal(1))
			actualInstanceID := fakeUAAClient.GetClientArgsForCall(0)
			Expect(actualInstanceID).To(Equal(instanceID))
		})

		When("the client exists", func() {
			It("returns the existing client", func() {
				fakeUAAClient.GetClientReturns(expectedClient, nil)

				actualClient, err := b.GetServiceInstanceClient(instanceID, rawContext)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualClient).To(Equal(expectedClient))
			})

			It("returns an error when getting fails", func() {
				fakeUAAClient.GetClientReturns(expectedClient, errors.New("failure"))

				_, err := b.GetServiceInstanceClient(instanceID, rawContext)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("failure"))
			})
		})

		When("the client does not exist", func() {
			It("creates a new client", func() {
				_, err := b.GetServiceInstanceClient(instanceID, rawContext)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeUAAClient.CreateClientCallCount()).To(Equal(1))
				actualInstanceID, actualInstanceName := fakeUAAClient.CreateClientArgsForCall(0)

				Expect(actualInstanceID).To(Equal(instanceID))
				Expect(actualInstanceName).To(Equal(instanceName))
			})

			It("returns a newly created client", func() {
				fakeUAAClient.CreateClientReturns(expectedClient, nil)

				actualClient, err := b.GetServiceInstanceClient(instanceID, rawContext)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualClient).To(Equal(expectedClient))
			})

			It("returns an error when creating fails", func() {
				fakeUAAClient.CreateClientReturns(nil, errors.New("create failed"))

				_, err := b.GetServiceInstanceClient(instanceID, rawContext)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("create failed"))
			})
		})
	})

	Describe("#UpdateServiceInstanceClient", func() {
		When("the current client is nil", func() {
			It("returns no error", func() {
				err := b.UpdateServiceInstanceClient(instanceID, nil, existingPlan, nil, logger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("there's a client to be updated", func() {
			var manifest []byte

			BeforeEach(func() {
				manifest = []byte("name: some-deployment")
			})

			It("regenerates the dashboard url", func() {
				err := b.UpdateServiceInstanceClient(instanceID, expectedClient, existingPlan, manifest, logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(serviceAdapter.GenerateDashboardUrlCallCount()).To(Equal(1))
				actualInstanceID, actualPlan, actualManifest, _ := serviceAdapter.GenerateDashboardUrlArgsForCall(0)
				Expect(actualInstanceID).To(Equal(instanceID))

				globalProperties := sdk.Properties{
					"some_other_global_property": "other_global_value",
					"a_global_property":          "global_value",
					"super":                      "no",
				}
				Expect(actualPlan).To(Equal(existingPlan.AdapterPlan(globalProperties)))
				Expect(actualManifest).To(Equal(manifest))
			})

			It("updates the client on UAA", func() {
				expectedRedirectURI := "http://some.example.com/dashboard"
				serviceAdapter.GenerateDashboardUrlReturns(expectedRedirectURI, nil)

				err := b.UpdateServiceInstanceClient(instanceID, expectedClient, existingPlan, manifest, logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeUAAClient.UpdateClientCallCount()).To(Equal(1))
				actualID, actualRedirectURI := fakeUAAClient.UpdateClientArgsForCall(0)
				Expect(actualID).To(Equal(instanceID))
				Expect(actualRedirectURI).To(Equal(expectedRedirectURI))
			})

			When("updating the client fails", func() {
				It("returns an error", func() {
					fakeUAAClient.UpdateClientReturns(nil, errors.New("update failed"))
					err := b.UpdateServiceInstanceClient(instanceID, expectedClient, existingPlan, manifest, logger)
					Expect(err).To(MatchError("update failed"))
				})
			})

			When("generating the dashboard url fails", func() {
				It("returns an error", func() {
					serviceAdapter.GenerateDashboardUrlReturns("", errors.New("dashboard failed"))

					err := b.UpdateServiceInstanceClient(instanceID, expectedClient, existingPlan, manifest, logger)
					Expect(err).To(MatchError("dashboard failed"))
				})
			})

			When("the adapter doesn't implement generate dashboard", func() {
				It("returns an error", func() {
					serviceAdapter.GenerateDashboardUrlReturns("", serviceadapter.NewNotImplementedError("not implemented"))

					err := b.UpdateServiceInstanceClient(instanceID, expectedClient, existingPlan, manifest, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeUAAClient.UpdateClientCallCount()).To(Equal(0))
				})
			})
		})
	})
})
