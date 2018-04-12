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
	"github.com/pivotal-cf/brokerapi"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Secure Binding", func() {
	const (
		instanceID = "some-instance-id"
		bindingID  = "some-binding-id"
	)

	var (
		bindDetails brokerapi.BindDetails
		expectedRef string
	)

	BeforeEach(func() {
		conf := brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
			},
			CredHub: brokerConfig.CredHub{
				APIURL: "https://fake.example.com",
			},
		}

		fakeBoshClient.GetDeploymentReturns([]byte(`name: 123`), true, nil)

		bindDetails = brokerapi.BindDetails{
			PlanID:    "plan-id",
			ServiceID: "service-id",
			AppGUID:   "app-guid",
			BindResource: &brokerapi.BindResource{
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
			fakeServiceAdapter.CreateBindingReturns(bindings, nil)

			fakeCredentialStore.SetReturns(nil)

			response, bodyContent := doBindRequest(instanceID, bindingID, bindDetails)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusCreated))

			By("calling bind on the adapter")
			Expect(fakeServiceAdapter.CreateBindingCallCount()).To(Equal(1))

			By("calling credhub")
			Expect(fakeCredentialStore.SetCallCount()).To(Equal(1))
			key, credentials := fakeCredentialStore.SetArgsForCall(0)
			Expect(key).To(Equal(expectedRef))
			Expect(credentials).To(Equal(bindings.Credentials))

			Expect(fakeCredentialStore.AddPermissionsCallCount()).To(Equal(1))
			key, additionalPermissions := fakeCredentialStore.AddPermissionsArgsForCall(0)
			Expect(key).To(Equal(expectedRef))
			Expect(additionalPermissions).To(HaveLen(1))
			Expect(additionalPermissions[0].Actor).To(Equal("mtls-app:app-guid"))
			Expect(additionalPermissions[0].Operations).To(Equal([]string{"read"}))

			By("returning the correct binding metadata")
			var responseBody brokerapi.Binding
			Expect(json.Unmarshal(bodyContent, &responseBody)).To(Succeed())
			Expect(responseBody).To(Equal(brokerapi.Binding{
				Credentials:     map[string]interface{}{"credhub-ref": expectedRef},
				SyslogDrainURL:  "other.fqdn",
				RouteServiceURL: "some.fqdn",
				VolumeMounts:    nil,
			}))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`storing credentials for instance ID`))
		})

		It("fails when cannot set credentials in credhub", func() {
			fakeCredentialStore.SetReturns(errors.New("oops"))

			By("retuning the correct status code")
			resp, _ := doBindRequest(instanceID, bindingID, bindDetails)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("fails when the adapter fails", func() {
			fakeServiceAdapter.CreateBindingReturns(sdk.Binding{}, errors.New("oops"))

			By("retuning the correct status code")
			resp, _ := doBindRequest(instanceID, bindingID, bindDetails)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("not calling credhub")
			Expect(fakeCredentialStore.SetCallCount()).To(Equal(0))
		})
	})

	Describe("unbinding", func() {
		It("attempts to remove the credentials from credhub", func() {
			By("retuning the correct status code")
			fakeCredentialStore.DeleteReturns(nil)
			resp, _ := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			By("calling unbind on the adapter")
			Expect(fakeServiceAdapter.DeleteBindingCallCount()).To(Equal(1))

			By("calling credhub")
			Expect(fakeCredentialStore.DeleteCallCount()).To(Equal(1))
			key := fakeCredentialStore.DeleteArgsForCall(0)
			Expect(key).To(Equal(expectedRef))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`removing credentials for instance ID`))
		})

		It("logs a warning if cannot remove the credentials in credhub", func() {
			By("retuning the correct status code")
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
			fakeServiceAdapter.DeleteBindingReturns(errors.New("oops"))
			By("retuning the correct status code")
			resp, _ := doUnbindRequest(instanceID, bindingID, serviceID, dedicatedPlanID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("not calling credhub")
			Expect(fakeCredentialStore.DeleteCallCount()).To(Equal(0))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(SatisfyAll(
				gbytes.Say(`removing credentials for instance ID`),
			))
		})
	})
})
