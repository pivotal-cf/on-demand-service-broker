package on_demand_service_broker_test

import (
	"fmt"

	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pkg/errors"

	"net/http"

	"encoding/json"

	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Unbind", func() {
	const (
		instanceID = "some-id"
		bindingID  = "some-binding-id"
	)

	var (
		boshManifest []byte
		boshVMs      bosh.BoshVMs
	)

	BeforeEach(func() {
		conf := brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
				Plans: brokerConfig.Plans{
					{
						Name: dedicatedPlanName,
						ID:   dedicatedPlanID,
					},
				},
			},
		}
		boshManifest = []byte(`name: 123`)
		boshVMs = bosh.BoshVMs{"some-property": []string{"some-value"}}
		fakeBoshClient.GetDeploymentReturns(boshManifest, true, nil)
		fakeBoshClient.VMsReturns(boshVMs, nil)

		StartServer(conf)
	})

	It("successfully unbind the service instance", func() {
		By("retuning the correct status code")
		resp := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		By("calling the adapter with the correct arguments")
		id, deploymentTopology, manifest, requestParams, _ := fakeServiceAdapter.DeleteBindingArgsForCall(0)
		Expect(id).To(Equal(bindingID))
		Expect(deploymentTopology).To(Equal(boshVMs))
		Expect(manifest).To(Equal(boshManifest))
		Expect(requestParams).To(Equal(map[string]interface{}{
			"plan_id":    dedicatedPlanID,
			"service_id": serviceID,
		}))

		By("logging the unbind request")
		Expect(loggerBuffer).To(gbytes.Say("service adapter will delete binding with ID"))
	})

	Describe("the failure scenarios", func() {
		It("responds with 500 and a generic message", func() {
			fakeServiceAdapter.DeleteBindingReturns(errors.New("oops"))

			resp := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)

			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(resp.Body).Decode(&errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: unbind"),
				Not(ContainSubstring("task-id:")),
			))
		})

		It("responds with 500 and a descriptive message", func() {
			fakeServiceAdapter.DeleteBindingReturns(serviceadapter.NewUnknownFailureError("error message for user"))
			resp := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(resp.Body).Decode(&errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(ContainSubstring("error message for user"))
		})

		It("responds with 410 when cannot find the binding", func() {
			fakeServiceAdapter.DeleteBindingReturns(serviceadapter.ErrorForExitCode(sdk.BindingNotFoundErrorExitCode, "error message for user"))

			resp := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusGone))

			defer resp.Body.Close()
			Expect(ioutil.ReadAll(resp.Body)).To(MatchJSON(`{}`))
		})

		It("responds with 410 when a non existing instance is unbound", func() {
			fakeBoshClient.GetDeploymentReturns(boshManifest, false, nil)

			resp := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusGone))
		})

		It("responds with 500 when adapter does not implement binder", func() {
			fakeServiceAdapter.DeleteBindingReturns(serviceadapter.ErrorForExitCode(sdk.NotImplementedExitCode, "error message for user"))
			resp := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(resp.Body).Decode(&errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: unbind"),
				Not(ContainSubstring("task-id:")),
			))

			Eventually(loggerBuffer).Should(gbytes.Say("delete binding: command not implemented by service adapter"))

		})

		It("responds with 500 when bosh fails", func() {
			fakeBoshClient.GetDeploymentReturns(nil, false, errors.New("some bosh error"))

			resp := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(resp.Body).Decode(&errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: unbind"),
			))
		})

		It("responds with 500 when bosh is unavailable", func() {
			fakeBoshClient.GetInfoReturns(boshdirector.Info{}, errors.New("bosh offline"))

			resp := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(resp.Body).Decode(&errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(
				ContainSubstring(
					"Currently unable to unbind service instance, please try again later",
				),
			)
		})

	})
})

func doUnbindRequest(instanceID, bindingID, serviceID, planID string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf(
			"http://%s/v2/service_instances/%s/service_bindings/%s?service_id=%s&plan_id=%s",
			serverURL,
			instanceID,
			bindingID,
			serviceID,
			planID,
		),
		nil)
	Expect(err).ToNot(HaveOccurred())
	req.SetBasicAuth(brokerUsername, brokerPassword)

	response, err := http.DefaultClient.Do(req)
	Expect(err).ToNot(HaveOccurred())

	return response
}
