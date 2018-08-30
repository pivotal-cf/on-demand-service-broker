// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"net/http"

	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("Basic authentication for BOSH", func() {
	Context("when the broker is configured to use basic authentication for BOSH", func() {
		var (
			boshDirector      *mockbosh.MockBOSH
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

			adapter.DashboardUrlGenerator().NotImplemented()

			conf = defaultBrokerConfig(boshDirector.URL, "UAA is not used", cfAPI.URL, cfUAA.URL)

		})

		AfterEach(func() {
			boshDirector.VerifyMocks()
			boshDirector.Close()

			cfAPI.VerifyMocks()
			cfAPI.Close()
			cfUAA.Close()
		})

		Context("happy path", func() {
			AfterEach(func() {
				killBrokerAndCheckForOpenConnections(runningBroker, boshDirector.URL)
			})

			It("starts and obtains a token only for CF", func() {
				boshDirector.ExpectedBasicAuth(boshUsername, boshPassword)
				boshDirector.ExcludeAuthorizationCheck("/info")
				conf.Bosh.Authentication = config.Authentication{
					Basic: config.UserCredentials{
						Username: boshUsername,
						Password: boshPassword,
					},
				}
				manifestYAML := rawManifestWithDeploymentName(instanceID)
				adapter.GenerateManifest().ToReturnManifest(manifestYAML)
				runningBroker = startBasicBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsOKWith(`{}`),
					mockbosh.Deployments().RespondsOKWith(fmt.Sprintf(`[{"Name": "not-the-one"}]`)),
					mockbosh.Tasks(deploymentName("some-instance-id")).RespondsWithNoTasks(),
					mockbosh.Deploy().RedirectsToTask(101),
					mockbosh.Task(101).RespondsWithTaskContainingState("in progress"),
					mockbosh.Task(101).RespondsWithTaskContainingState("done"),
					mockbosh.TaskOutputEvent(101).RespondsWithTaskOutput([]boshdirector.BoshTaskOutput{}),
					mockbosh.TaskOutput(101).RespondsWithTaskOutput([]boshdirector.BoshTaskOutput{}),
				)
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(dedicatedPlanID, "ff717e7c-afd5-4d0a-bafe-16c7eff546ec"),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
				)
				provisionResponse = provisionInstance(instanceID, dedicatedPlanID, map[string]interface{}{})

				Expect(cfUAA.TokensIssued).To(Equal(1))
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))
			})
		})

		It("fails to start if the Basic Auth credentials are incorrect", func() {
			conf.Bosh.Authentication = config.Authentication{
				Basic: config.UserCredentials{
					Username: "bad-username",
					Password: "bad-password",
				},
			}
			manifestYAML := rawManifestWithDeploymentName(instanceID)
			adapter.GenerateManifest().ToReturnManifest(manifestYAML)
			boshDirector.VerifyAndMock(
				mockbosh.Info().RespondsOKForBasicAuth(),
				mockbosh.Info().RespondsOKForBasicAuth(),
				mockbosh.Info().RespondsUnauthorizedWith("{}"),
				mockbosh.Info().RespondsOKForBasicAuth(),
			)
			cfAPI.VerifyAndMock(
				mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
				mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
			)

			runningBroker = startBrokerWithoutPortCheck(conf)

			Eventually(runningBroker).Should(gexec.Exit())
			Expect(runningBroker.ExitCode()).ToNot(Equal(0))
		})
	})
})
