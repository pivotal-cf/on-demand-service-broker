// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package on_demand_service_broker_test

import (
	"bytes"
	"errors"
	"fmt"

	"net/http"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Binding", func() {
	const (
		instanceID = "some-instance-id"
		bindingID  = "some-binding-id"
	)

	var (
		expectedDeploymentName = fmt.Sprintf("service-instance_%s", instanceID)
		boshManifest           []byte
		boshVMs                bosh.BoshVMs
		bindDetails            brokerapi.BindDetails
		bindingRequestDetails  map[string]interface{}
	)

	BeforeEach(func() {
		conf := brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
			},
		}

		boshManifest = []byte(`name: 123`)
		boshVMs = bosh.BoshVMs{"some-property": []string{"some-value"}}
		fakeBoshClient.VMsReturns(boshVMs, nil)
		fakeBoshClient.GetDeploymentReturns(boshManifest, true, nil)

		bindDetails = brokerapi.BindDetails{
			PlanID:    "plan-id",
			ServiceID: "service-id",
			AppGUID:   "app-guid",
			BindResource: &brokerapi.BindResource{
				AppGuid: "app-guid",
			},
			RawParameters: []byte(`{"baz": "bar"}`),
		}

		bindingRequestDetails = map[string]interface{}{
			"plan_id":    bindDetails.PlanID,
			"service_id": bindDetails.ServiceID,
			"app_guid":   bindDetails.AppGUID,
			"bind_resource": map[string]interface{}{
				"app_guid": bindDetails.AppGUID,
			},
			"parameters": map[string]interface{}{"baz": "bar"},
		}

		StartServer(conf)
	})

	Describe("a successful binding", func() {
		It("generates a meaningful response", func() {
			fakeServiceAdapter.CreateBindingReturns(sdk.Binding{
				Credentials:     map[string]interface{}{"user": "bill", "password": "redflag"},
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
			}, nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("fetching the VM info for the deployment")
			deploymentName, _ := fakeBoshClient.VMsArgsForCall(0)
			Expect(deploymentName).To(Equal(expectedDeploymentName))

			By("fetching the deployment manifest")
			deploymentName, _ = fakeBoshClient.GetDeploymentArgsForCall(0)
			Expect(deploymentName).To(Equal(expectedDeploymentName))

			By("calling bind on the service adapter")
			Expect(fakeServiceAdapter.CreateBindingCallCount()).To(Equal(1))
			id, vms, manifest, params, _ := fakeServiceAdapter.CreateBindingArgsForCall(0)
			Expect(id).To(Equal(bindingID))
			Expect(vms).To(Equal(boshVMs))
			Expect(manifest).To(Equal(boshManifest))
			Expect(params).To(Equal(bindingRequestDetails))

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusCreated))

			By("returning the correct binding metadata")
			var responseBody brokerapi.Binding
			Expect(json.Unmarshal(bodyContent, &responseBody)).To(Succeed())
			Expect(responseBody).To(Equal(brokerapi.Binding{
				Credentials:     map[string]interface{}{"user": "bill", "password": "redflag"},
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
				VolumeMounts:    nil,
			}))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`service adapter will create binding with ID`))
		})

		It("generates a body without missing optional fields", func() {
			fakeServiceAdapter.CreateBindingReturns(sdk.Binding{
				Credentials:     map[string]interface{}{"user": "bill", "password": "redflag"},
				SyslogDrainURL:  "",
				RouteServiceURL: "",
			}, nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning 201")
			Expect(response.StatusCode).To(Equal(http.StatusCreated))

			By("returning the correct binding metadata")
			var responseBody map[string]interface{}
			Expect(json.Unmarshal(bodyContent, &responseBody)).To(Succeed())

			Expect(responseBody).To(HaveKey("credentials"))
			Expect(responseBody).NotTo(HaveKey("syslog_drain_url"))
			Expect(responseBody).NotTo(HaveKey("route_service_url"))
		})
	})

	Describe("non-successful binding", func() {
		It("returns 500 if cannot fetch VMs info", func() {
			fakeBoshClient.VMsReturns(nil, errors.New("oops"))

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: bind"),
			))
		})

		It("responds with status 500 and a try-again-later message when the bosh director is unavailable", func() {
			fakeBoshClient.GetInfoReturns(boshdirector.Info{}, errors.New("oops"))

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(ContainSubstring("Currently unable to bind service instance, please try again later"))
		})

		It("responds with status 404 when the deployment does not exist", func() {
			fakeBoshClient.GetDeploymentReturns(nil, false, nil)
			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(ContainSubstring("instance does not exist"))
		})

		It("responds with status 500 and a generic message when talking to bosh fails", func() {
			fakeBoshClient.GetDeploymentReturns(nil, false, errors.New("oops"))
			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: bind"),
			))
		})

		It("responds with status 409 if the binding already exists", func() {
			err := serviceadapter.ErrorForExitCode(sdk.BindingAlreadyExistsErrorExitCode, "oops")
			fakeServiceAdapter.CreateBindingReturns(
				sdk.Binding{},
				err,
			)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusConflict))

			By("returning the correct error message")
			Expect(bodyContent).To(MatchJSON(`{"description":"binding already exists"}`))

			By("logging the bind request with a request id")
			Eventually(loggerBuffer).Should(gbytes.Say(`creating binding: binding already exists`))
		})

		It("responds with status 422 if the app GUID was not provided", func() {
			err := serviceadapter.ErrorForExitCode(sdk.AppGuidNotProvidedErrorExitCode, "oops")
			fakeServiceAdapter.CreateBindingReturns(
				sdk.Binding{},
				err,
			)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusUnprocessableEntity))

			By("returning the correct error message")
			Expect(bodyContent).To(MatchJSON(`{"description":"app_guid is a required field but was not provided"}`))
		})

		It("responds with status 500 when the adapter does not implement binder", func() {
			err := serviceadapter.ErrorForExitCode(sdk.NotImplementedExitCode, "oops")
			fakeServiceAdapter.CreateBindingReturns(
				sdk.Binding{},
				err,
			)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: bind"),
			))

			By("logging an appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(`creating binding: command not implemented`))
		})

		It("responds with status 500 when the adapter fails with unknown error", func() {
			err := serviceadapter.NewUnknownFailureError("")
			fakeServiceAdapter.CreateBindingReturns(
				sdk.Binding{},
				err,
			)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: bind"),
			))
		})
	})
})

func doBindRequest(instanceID, bindingID string, bindDetails brokerapi.BindDetails) (*http.Response, []byte) {
	body := bytes.NewBuffer([]byte{})
	err := json.NewEncoder(body).Encode(bindDetails)
	Expect(err).NotTo(HaveOccurred())

	return doRequest(
		http.MethodPut,
		fmt.Sprintf(
			"http://%s/v2/service_instances/%s/service_bindings/%s",
			serverURL,
			instanceID,
			bindingID,
		),
		body,
	)
}
