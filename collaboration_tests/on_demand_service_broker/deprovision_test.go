package on_demand_service_broker_test

import (
	"errors"
	"fmt"
	"net/http"

	"encoding/json"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Deprovision", func() {
	const (
		instanceID = "some-instance-id"
		taskID     = 42
	)

	BeforeEach(func() {
		boshManifest := []byte(`name: 123`)
		boshVMs := bosh.BoshVMs{"some-property": []string{"some-value"}}
		fakeBoshClient.VMsReturns(boshVMs, nil)
		fakeBoshClient.GetDeploymentReturns(boshManifest, true, nil)
		fakeBoshClient.DeleteDeploymentReturns(taskID, nil)
	})

	Context("without pre-delete errand", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}

			StartServer(conf)
		})

		It("succeeds with async flag", func() {
			response := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			By("including the operation data in the response")
			var deprovisionResponse brokerapi.DeprovisionResponse
			err = json.Unmarshal(body, &deprovisionResponse)
			Expect(err).NotTo(HaveOccurred())

			var operationData broker.OperationData
			err = json.Unmarshal([]byte(deprovisionResponse.OperationData), &operationData)
			Expect(err).NotTo(HaveOccurred())

			Expect(operationData.OperationType).To(Equal(broker.OperationTypeDelete))
			Expect(operationData.BoshTaskID).To(Equal(taskID))
			Expect(operationData.BoshContextID).To(BeEmpty())

			By("logging the delete request")
			Eventually(loggerBuffer).Should(gbytes.Say(`deleting deployment for instance`))
		})

		It("fails when async flag is not set", func() {
			response := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, false)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusUnprocessableEntity))

			By("returning an informative error")
			var respStructure map[string]interface{}
			Expect(json.NewDecoder(response.Body).Decode(&respStructure)).To(Succeed())

			Expect(respStructure).To(Equal(map[string]interface{}{
				"error":       "AsyncRequired",
				"description": "This service plan requires client support for asynchronous service operations.",
			}))
		})
	})

	Context("with pre-delete errand", func() {
		const (
			preDeleteErrandPlanID = "pre-delete-errand-id"
			errandTaskID          = 187
		)

		BeforeEach(func() {
			errandName := "cleanup-resources"

			preDeleteErrandPlan := brokerConfig.Plan{
				Name: "pre-delete-errand-plan",
				ID:   preDeleteErrandPlanID,
				InstanceGroups: []sdk.InstanceGroup{
					{
						Name:      "instance-group-name",
						VMType:    "pre-delete-errand-vm-type",
						Instances: 1,
						Networks:  []string{"net1"},
						AZs:       []string{"az1"},
					},
				},
				LifecycleErrands: &sdk.LifecycleErrands{
					PreDelete: sdk.Errand{Name: errandName},
				},
			}

			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name:  serviceName,
					Plans: brokerConfig.Plans{preDeleteErrandPlan},
				},
			}

			fakeBoshClient.RunErrandReturns(errandTaskID, nil)
			StartServer(conf)
		})

		It("succeeds with async flag", func() {
			response := doDeprovisionRequest(instanceID, preDeleteErrandPlanID, serviceID, true)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			By("including the operation data in the response")
			var deprovisionResponse brokerapi.DeprovisionResponse
			err = json.Unmarshal(body, &deprovisionResponse)
			Expect(err).NotTo(HaveOccurred())

			var operationData broker.OperationData
			err = json.Unmarshal([]byte(deprovisionResponse.OperationData), &operationData)
			Expect(err).NotTo(HaveOccurred())

			Expect(operationData.OperationType).To(Equal(broker.OperationTypeDelete))
			Expect(operationData.BoshTaskID).To(Equal(errandTaskID))
			Expect(operationData.BoshContextID).To(MatchRegexp(
				`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			))

			By("logging the delete request")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf("running pre-delete errand for instance %s", instanceID),
			))
		})
	})

	Context("the failure scenarios", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}
			StartServer(conf)
		})

		It("returns 410 when the deployment does not exist", func() {
			fakeBoshClient.GetDeploymentReturns(nil, false, nil)

			response := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusGone))

			By("returning no body")
			Expect(ioutil.ReadAll(response.Body)).To(MatchJSON("{}"))

			By("logging the delete request")
			Eventually(loggerBuffer).Should(
				gbytes.Say(fmt.Sprintf("error deprovisioning: instance %s, not found.", instanceID)),
			)
		})

		It("returns 500 when a BOSH task is in flight", func() {
			task := boshdirector.BoshTask{
				ID:    99,
				State: boshdirector.TaskProcessing,
			}
			tasks := boshdirector.BoshTasks{task}
			fakeBoshClient.GetTasksReturns(tasks, nil)

			response := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error data")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(response.Body).Decode(&errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(ContainSubstring(
				"An operation is in progress for your service instance. Please try again later.",
			))

			By("logging the delete request")
			Eventually(loggerBuffer).Should(
				gbytes.Say(fmt.Sprintf("deployment service-instance_%s is still in progress:", instanceID)),
			)
		})

		It("returns 500 when the BOSH director is unavailable", func() {
			fakeBoshClient.GetInfoReturns(boshdirector.Info{}, errors.New("oops"))

			response := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error data")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(response.Body).Decode(&errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(ContainSubstring("Currently unable to delete service instance, please try again later"))
		})
	})
})

func doDeprovisionRequest(instanceID, planID, serviceID string, asyncAllowed bool) *http.Response {
	deprovisionReq, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=%t&plan_id=%s&service_id=%s", serverURL, instanceID, asyncAllowed, planID, serviceID),
		nil)
	Expect(err).ToNot(HaveOccurred())
	deprovisionReq.Header.Set("X-Broker-API-Version", "2.0")
	deprovisionReq.SetBasicAuth(brokerUsername, brokerPassword)
	deprovisionResponse, err := http.DefaultClient.Do(deprovisionReq)
	Expect(err).ToNot(HaveOccurred())
	return deprovisionResponse
}
