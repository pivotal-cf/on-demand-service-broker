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
	"github.com/pivotal-cf/brokerapi/v8/domain"
	"github.com/pivotal-cf/brokerapi/v8/domain/apiresponses"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
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
		bindDetails            domain.BindDetails
		bindingRequestDetails  map[string]interface{}
		bindingWithDNSConf     []brokerConfig.BindingDNS
		bindingWithDNSDetails  map[string]string
	)

	BeforeEach(func() {
		bindingWithDNSConf = []brokerConfig.BindingDNS{
			{
				Name:          "config-1",
				LinkProvider:  "link-provider",
				InstanceGroup: "instance-group",
				Properties: brokerConfig.BindingDNSProperties{
					AZS:    []string{"europe-a1", "antartica-z1"},
					Status: "healthy",
				},
			},
		}
		bindingWithDNSDetails = map[string]string{
			"config-1": "some.names.bosh",
		}

		conf := brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
				Plans: []brokerConfig.Plan{
					{
						ID:             "plan-id",
						BindingWithDNS: bindingWithDNSConf,
					},
				},
			},
		}

		boshManifest = []byte(`name: 123`)
		boshVMs = bosh.BoshVMs{"some-property": []string{"some-value"}}
		fakeBoshClient.VMsReturns(boshVMs, nil)
		fakeBoshClient.GetDeploymentReturns(boshManifest, true, nil)

		bindDetails = domain.BindDetails{
			PlanID:    "plan-id",
			ServiceID: "service-id",
			AppGUID:   "app-guid",
			BindResource: &domain.BindResource{
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
			fakeBoshClient.GetDNSAddressesReturns(bindingWithDNSDetails, nil)
			bindingJSON := toJson(sdk.Binding{
				Credentials:     map[string]interface{}{"user": "bill", "password": "redflag"},
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
			})
			zero := 0
			fakeCommandRunner.RunWithInputParamsReturns(bindingJSON, []byte{}, &zero, nil)
			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("fetching the VM info for the deployment")
			deploymentName, _ := fakeBoshClient.VMsArgsForCall(0)
			Expect(deploymentName).To(Equal(expectedDeploymentName))

			By("fetching the deployment manifest")
			deploymentName, _ = fakeBoshClient.GetDeploymentArgsForCall(0)
			Expect(deploymentName).To(Equal(expectedDeploymentName))

			By("fetching the Bosh DNS config")
			dnsDeploymentName, bindingBoshDNSConfig := fakeBoshClient.GetDNSAddressesArgsForCall(0)
			Expect(dnsDeploymentName).To(Equal(expectedDeploymentName))
			Expect(bindingBoshDNSConfig).To(Equal(bindingWithDNSConf))

			By("calling bind on the service adapter")
			args := getBindInputParams()

			Expect(args.BindingId).To(Equal(bindingID))
			Expect(args.BoshVms).To(Equal(string(toJson(boshVMs))))
			Expect(args.Manifest).To(Equal(string(boshManifest)))
			Expect(args.RequestParameters).To(Equal(string(toJson(bindingRequestDetails))))
			Expect(args.DNSAddresses).To(Equal(string(toJson(bindingWithDNSDetails))))

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusCreated))

			By("returning the correct binding metadata")
			var responseBody domain.Binding
			Expect(json.Unmarshal(bodyContent, &responseBody)).To(Succeed())
			Expect(responseBody).To(Equal(domain.Binding{
				Credentials:     map[string]interface{}{"user": "bill", "password": "redflag"},
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
				VolumeMounts:    nil,
			}))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`service adapter will create binding with ID`))
		})

		It("generates a body without missing optional fields", func() {
			bindingJSON := toJson(sdk.Binding{
				Credentials:     map[string]interface{}{"user": "bill", "password": "redflag"},
				SyslogDrainURL:  "",
				RouteServiceURL: "",
			})
			zero := 0
			fakeCommandRunner.RunWithInputParamsReturns(bindingJSON, []byte{}, &zero, nil)

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

		Context("Credhub Secrets", func() {
			When("the secret is not in the variables block", func() {
				It("returns a secret map with the secret resolved", func() {
					credhubPath := "/path/to/foo"
					credhubSecret := "credhub-secret"
					credhubRef := fmt.Sprintf("((%s))", credhubPath)
					boshManifest = []byte(fmt.Sprintf("---name: 123\nbob: %s", credhubRef))

					expectedSecrets := map[string]string{
						credhubPath: credhubSecret,
					}
					fakeCredhubOperator.BulkGetReturns(expectedSecrets, nil)
					fakeBoshClient.GetDeploymentReturns(boshManifest, true, nil)

					// fake bosh credhub to respond to path /path/to/something with foobar
					doBindRequest(instanceID, bindingID, bindDetails)
					args := getBindInputParams()
					Expect(args.Secrets).To(Equal(string(toJson(expectedSecrets))))
				})
			})
			When("the secret is in the variables block", func() {
				It("returns a secret map with the secret resolved", func() {
					boshManifest = []byte(`---
name: banana
password: ((supersecret))
variables:
- name: supersecret
  type: password`)

					expectedSecrets := map[string]string{
						"supersecret": "some-secret-value",
					}
					fakeCredhubOperator.BulkGetReturns(expectedSecrets, nil)
					fakeBoshClient.GetDeploymentReturns(boshManifest, true, nil)

					doBindRequest(instanceID, bindingID, bindDetails)
					args := getBindInputParams()
					Expect(args.Secrets).To(Equal(string(toJson(expectedSecrets))))
				})
			})
		})
	})

	Describe("non-successful binding", func() {
		It("returns 500 if cannot fetch VMs info", func() {
			fakeBoshClient.VMsReturns(nil, errors.New("oops"))

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse apiresponses.ErrorResponse
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

		It("responds with status 404 when the deployment does not exist", func() {
			fakeBoshClient.GetDeploymentReturns(nil, false, nil)
			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))

			By("returning the correct error message")
			var errorResponse apiresponses.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())

			Expect(errorResponse.Description).To(ContainSubstring("instance does not exist"))
		})

		It("responds with status 500 and a generic message when talking to bosh fails", func() {
			fakeBoshClient.GetDeploymentReturns(nil, false, errors.New("oops"))
			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse apiresponses.ErrorResponse
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

		It("responds with status 500 and a generic message when getting bosh DNS fails", func() {
			fakeBoshClient.GetDNSAddressesReturns(nil, errors.New("oops"))
			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error message")
			var errorResponse apiresponses.ErrorResponse
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
			errCode := sdk.BindingAlreadyExistsErrorExitCode
			fakeCommandRunner.RunWithInputParamsReturns([]byte("oops"), []byte{}, &errCode, nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusConflict))

			By("returning the correct error message")
			Expect(bodyContent).To(MatchJSON(`{"description":"binding already exists"}`))

			By("logging the bind request with a request id")
			Eventually(loggerBuffer).Should(gbytes.Say(`creating binding: binding already exists`))
		})

		It("responds with status 422 if the app GUID was not provided", func() {
			errCode := sdk.AppGuidNotProvidedErrorExitCode
			fakeCommandRunner.RunWithInputParamsReturns([]byte("oops"), []byte{}, &errCode, nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusUnprocessableEntity))

			By("returning the correct error message")
			Expect(bodyContent).To(MatchJSON(`{"description":"app_guid is a required field but was not provided"}`))
		})

		It("responds with status 500 when the adapter does not implement binder", func() {
			errCode := sdk.NotImplementedExitCode
			fakeCommandRunner.RunWithInputParamsReturns([]byte("oops"), []byte{}, &errCode, nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			var errorResponse apiresponses.ErrorResponse
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
			errCode := 2
			fakeCommandRunner.RunWithInputParamsReturns([]byte{}, []byte{}, &errCode, nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			var errorResponse apiresponses.ErrorResponse
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

func doBindRequest(instanceID, bindingID string, bindDetails domain.BindDetails) (*http.Response, []byte) {
	body := bytes.NewBuffer([]byte{})
	err := json.NewEncoder(body).Encode(bindDetails)
	Expect(err).NotTo(HaveOccurred())

	return doRequestWithAuthAndHeaderSet(
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

func getBindInputParams() sdk.CreateBindingJSONParams {
	Expect(fakeCommandRunner.RunWithInputParamsCallCount()).To(Equal(1))
	input, varArgs := fakeCommandRunner.RunWithInputParamsArgsForCall(0)
	Expect(varArgs).To(HaveLen(2))
	Expect(varArgs[1]).To(Equal("create-binding"))
	inputParams, ok := input.(sdk.InputParams)
	Expect(ok).To(BeTrue(), "couldn't cast input to sdk.InputParams")
	return inputParams.CreateBinding
}
