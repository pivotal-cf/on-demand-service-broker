// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package on_demand_service_broker_test

import (
	"errors"
	"net/http"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/brokerapi/v8/domain"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Secure Binding", func() {
	const (
		instanceID = "some-instance-id"
		bindingID  = "some-binding-id"
	)

	var (
		bindDetails domain.BindDetails
		expectedRef string
	)

	BeforeEach(func() {
		conf := brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
				Plans: brokerConfig.Plans{
					brokerConfig.Plan{
						ID:   dedicatedPlanID,
						Name: "dedicated plan",
					},
				},
			},
			CredHub: brokerConfig.CredHub{
				APIURL: "https://fake.example.com",
			},
		}

		fakeBoshClient.GetDeploymentReturns([]byte(`name: 123`), true, nil)

		bindDetails = domain.BindDetails{
			PlanID:    "plan-id",
			ServiceID: "service-id",
			AppGUID:   "app-guid",
			BindResource: &domain.BindResource{
				AppGuid: "app-guid",
			},
			RawParameters: []byte(`{"baz": "bar"}`),
		}

		expectedRef = "/c/service-id/some-instance-id/some-binding-id/credentials"

		StartServer(conf)
	})

	Describe("binding", func() {
		It("sets the credentials in credhub and returns a reference", func() {
			bindings := sdk.Binding{
				Credentials:     map[string]interface{}{"user": "bill", "password": "redflag"},
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
			}
			var zero int
			bindingsJson := toJson(bindings)
			fakeCommandRunner.RunWithInputParamsReturns(bindingsJson, nil, &zero, nil)

			fakeCredentialStore.SetReturns(nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusCreated))

			By("calling bind on the adapter")
			Expect(fakeCommandRunner.RunWithInputParamsCallCount()).To(Equal(1))
			_, varArgs := fakeCommandRunner.RunWithInputParamsArgsForCall(0)
			Expect(varArgs).To(HaveLen(2))
			Expect(varArgs[1]).To(Equal("create-binding"))

			By("calling credhub")
			Expect(fakeCredentialStore.SetCallCount()).To(Equal(1))
			key, credentials := fakeCredentialStore.SetArgsForCall(0)
			Expect(key).To(Equal(expectedRef))
			Expect(credentials).To(Equal(bindings.Credentials))

			Expect(fakeCredentialStore.AddPermissionCallCount()).To(Equal(1))
			key, actor, ops := fakeCredentialStore.AddPermissionArgsForCall(0)
			Expect(key).To(Equal(expectedRef))
			Expect(actor).To(Equal("mtls-app:app-guid"))
			Expect(ops).To(Equal([]string{"read"}))

			By("returning the correct binding metadata")
			var responseBody domain.Binding
			Expect(json.Unmarshal(bodyContent, &responseBody)).To(Succeed())
			Expect(responseBody).To(Equal(domain.Binding{
				Credentials:     map[string]interface{}{"credhub-ref": expectedRef},
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
				VolumeMounts:    nil,
			}))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`storing credentials for instance ID`))
		})

		It("does not set empty credentials in credhub and returns a reference", func() {
			bindings := sdk.Binding{
				Credentials:     nil,
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
			}

			var zero int
			bindingsJson := toJson(bindings)
			fakeCommandRunner.RunWithInputParamsReturns(bindingsJson, nil, &zero, nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusCreated))

			By("calling bind on the adapter")
			Expect(fakeCommandRunner.RunWithInputParamsCallCount()).To(Equal(1))
			_, varArgs := fakeCommandRunner.RunWithInputParamsArgsForCall(0)
			Expect(varArgs).To(HaveLen(2))
			Expect(varArgs[1]).To(Equal("create-binding"))

			By("not calling credhub")
			Expect(fakeCredentialStore.SetCallCount()).To(Equal(0))
			Expect(fakeCredentialStore.AddPermissionCallCount()).To(Equal(0))

			By("returning the correct binding metadata")
			var responseBody domain.Binding
			Expect(json.Unmarshal(bodyContent, &responseBody)).To(Succeed())
			Expect(responseBody).To(Equal(domain.Binding{
				Credentials:     nil,
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
				VolumeMounts:    nil,
			}))
		})

		It("fails when cannot set credentials in credhub", func() {
			fakeCredentialStore.SetReturns(errors.New("oops"))

			By("returning the correct status code")
			resp, _ := doBindRequest(instanceID, bindingID, bindDetails)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("fails when the commandRunner returns an error", func() {
			var zero int
			fakeCommandRunner.RunWithInputParamsReturns([]byte{}, []byte{}, &zero, errors.New("commandRunner returned error"))

			By("returning the correct status code")
			resp, _ := doBindRequest(instanceID, bindingID, bindDetails)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			Eventually(loggerBuffer).Should(gbytes.Say("commandRunner returned error"))

			By("not calling credhub")
			Expect(fakeCredentialStore.SetCallCount()).To(Equal(0))
		})
	})

	Describe("unbinding", func() {
		It("attempts to remove the credentials from credhub", func() {
			By("returning the correct status code")
			fakeCredentialStore.DeleteReturns(nil)
			resp, _ := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			By("calling bind on the adapter")
			Expect(fakeCommandRunner.RunWithInputParamsCallCount()).To(Equal(1))
			_, varArgs := fakeCommandRunner.RunWithInputParamsArgsForCall(0)
			Expect(varArgs).To(HaveLen(2))
			Expect(varArgs[1]).To(Equal("delete-binding"))

			By("calling credhub")
			Expect(fakeCredentialStore.DeleteCallCount()).To(Equal(1))
			key := fakeCredentialStore.DeleteArgsForCall(0)
			Expect(key).To(Equal(expectedRef))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`removing credentials for instance ID`))
		})

		It("logs a warning if cannot remove the credentials in credhub", func() {
			By("returning the correct status code")
			fakeCredentialStore.DeleteReturns(errors.New("oops"))
			resp, _ := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(SatisfyAll(
				gbytes.Say(`removing credentials for instance ID`),
				gbytes.Say(`WARNING: failed to remove key`),
			))
		})

		It("fails when the adapter fails", func() {
			By("returning the correct status code")
			var zero int
			fakeCommandRunner.RunWithInputParamsReturns([]byte{}, []byte{}, &zero, errors.New("commandRunner returned error"))

			resp, _ := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("not calling credhub")
			Expect(fakeCredentialStore.DeleteCallCount()).To(Equal(0))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(SatisfyAll(
				gbytes.Say(`removing credentials for instance ID`),
				gbytes.Say("commandRunner returned error"),
			))
		})
	})
})
