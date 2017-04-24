// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Basic authentication for BOSH", func() {
	Context("when the broker is configured to use basic authentication for BOSH", func() {
		var (
			boshDirector      *mockhttp.Server
			cfAPI             *mockhttp.Server
			cfUAA             *mockuaa.ClientCredentialsServer
			conf              config.Config
			runningBroker     *gexec.Session
			provisionResponse *http.Response
			instanceID        = "some-instance-id"
		)

		BeforeEach(func() {
			boshDirector = mockbosh.New()
			cfAPI = mockcfapi.New()
			cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
			boshDirector.ExpectedBasicAuth(boshUsername, boshPassword)
			adapter.DashboardUrlGenerator().NotImplemented()

			conf = defaultBrokerConfig(boshDirector.URL, "UAA is not used", cfAPI.URL, cfUAA.URL)
			conf.Bosh.Authentication = config.BOSHAuthentication{
				Basic: config.UserCredentials{
					Username: boshUsername,
					Password: boshPassword,
				},
			}
		})

		JustBeforeEach(func() {
			manifestYAML := rawManifestWithDeploymentName(instanceID)
			adapter.GenerateManifest().ToReturnManifest(manifestYAML)
			runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)
			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName("some-instance-id")).NotFound(),
				mockbosh.Tasks(deploymentName("some-instance-id")).RespondsWithNoTasks(),
				mockbosh.Deploy().RedirectsToTask(101),
			)
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
				mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(dedicatedPlanID, "ff717e7c-afd5-4d0a-bafe-16c7eff546ec"),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsWith(listCFServiceInstanceCountForPlanResponse(0)),
			)
			provisionResponse = provisionInstance(instanceID, dedicatedPlanID, map[string]interface{}{})
		})

		AfterEach(func() {
			killBrokerAndCheckForOpenConnections(runningBroker, boshDirector.URL)
			boshDirector.VerifyMocks()
			boshDirector.Close()

			cfAPI.VerifyMocks()
			cfAPI.Close()
			cfUAA.Close()
		})

		It("obtains a token from the UAA", func() {
			Expect(cfUAA.TokensIssued).To(Equal(1))
		})

		It("succeeds", func() {
			Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))
		})
	})
})
