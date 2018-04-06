// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"os/exec"

	"time"

	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Startup", func() {
	var runningBroker *gexec.Session

	Context("when supplied with an invalid config file path", func() {
		BeforeEach(func() {
			// ensure that any previous instance of the broker has stopped
			// if this assertion fails, look for running instances of on-demand-broker and kill them!
			Eventually(dialBroker).Should(BeFalse(), "an old instance of the broker is still running")

			var err error
			runningBroker, err = gexec.Start(exec.Command(brokerBinPath, "-configFilePath", "/i_hope_this_does_not_exist_on_your_system"), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			runningBroker.Wait(time.Second)
		})

		It("exits with error", func() {
			Expect(runningBroker.ExitCode()).ToNot(Equal(0))
		})

		It("prints the error", func() {
			Expect(string(runningBroker.Buffer().Contents())).To(ContainSubstring("error parsing config"))
		})
	})

	Context("when service deployment has invalid versions", func() {
		var (
			boshDirector *mockbosh.MockBOSH
			boshUAA      *mockuaa.ClientCredentialsServer
			cfAPI        *mockhttp.Server
			cfUAA        *mockuaa.ClientCredentialsServer
			conf         config.Config
		)

		BeforeEach(func() {
			boshUAA = mockuaa.NewClientCredentialsServerTLS(boshClientID, boshClientSecret, pathToSSLCerts("cert.pem"), pathToSSLCerts("key.pem"), "bosh uaa token")
			boshDirector = mockbosh.NewWithUAA(boshUAA.URL)

			cfAPI = mockcfapi.New()
			cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")

			conf = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
		})

		AfterEach(func() {
			if runningBroker != nil {
				Eventually(runningBroker.Terminate()).Should(gexec.Exit())
			}
			boshDirector.VerifyMocks()
			boshDirector.Close()
			boshUAA.Close()
			cfAPI.VerifyMocks()
			cfAPI.Close()
			cfUAA.Close()
		})

		It("exits with error", func() {
			conf.ServiceDeployment.Stemcell.Version = "latest"
			runningBroker = startBrokerWithoutPortCheck(conf)
			Eventually(runningBroker).Should(gexec.Exit())
			Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			Eventually(runningBroker.Out).Should(gbytes.Say(
				"You must configure the exact release and stemcell versions in broker.service_deployment. ODB requires exact versions to detect pending changes as part of the 'cf update-service' workflow. For example, latest and 3112.latest are not supported.",
			))
		})
	})

	Describe("CF api", func() {
		var (
			boshDirector *mockbosh.MockBOSH
			boshUAA      *mockuaa.ClientCredentialsServer
			cfAPI        *mockhttp.Server
			cfUAA        *mockuaa.ClientCredentialsServer
			conf         config.Config
		)

		BeforeEach(func() {
			boshUAA = mockuaa.NewClientCredentialsServerTLS(boshClientID, boshClientSecret, pathToSSLCerts("cert.pem"), pathToSSLCerts("key.pem"), "bosh uaa token")
			boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
			boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
			boshDirector.ExcludeAuthorizationCheck("/info")

			cfAPI = mockcfapi.New()
			cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
			conf = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
			conf.Broker.StartUpBanner = true
		})

		AfterEach(func() {
			if runningBroker != nil {
				Eventually(runningBroker.Terminate()).Should(gexec.Exit())
			}
			boshDirector.VerifyMocks()
			boshDirector.Close()
			boshUAA.Close()
			cfAPI.VerifyMocks()
			cfAPI.Close()
			cfUAA.Close()
		})

		Context("when the configured service is not registered with Cloud Foundry", func() {
			It("doesn't fail", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
					mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
				)
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
				)

				runningBroker = startBroker(conf)

				odbLogPattern := `\[on-demand-service-broker\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}`

				Eventually(runningBroker.Out).Should(gbytes.Say(fmt.Sprintf(`%s Starting broker`, odbLogPattern)))
				Eventually(runningBroker.Out).Should(gbytes.Say(`-------./ssssssssssssssssssss:.-------`))
				Eventually(runningBroker.Out).Should(gbytes.Say(fmt.Sprintf(`%s Listening on :%d`, odbLogPattern, conf.Broker.Port)))
				Consistently(runningBroker).ShouldNot(gexec.Exit())
			})
		})

		Context("when there are service instances with plans that exist in the catalog", func() {
			It("doesn't fail at start up", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
					mockcfapi.ListServiceOfferings().RespondsOKWith(`{
				  "next_url": null,
				  "resources": [
				    {
				      "entity": {
				        "unique_id": "`+serviceID+`",
				        "service_plans_url": "/v2/services/06df08f9-5a58-4d33-8097-32d0baf3ce1e/service_plans"
				      }
				    }
				  ]
				}`),
					mockcfapi.ListServicePlans("06df08f9-5a58-4d33-8097-32d0baf3ce1e").RespondsOKWith(`{
				   "next_url": null,
				   "resources": [
				      {
				         "entity": {
								 		"unique_id": "`+dedicatedPlanID+`",
				            "service_instances_url": "/v2/service_plans/ff717e7c-afd5-4d0a-bafe-16c7eff546ec/service_instances"
				         }
				      }
				   ]
				}`),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(`{
				   "total_results": 1
				}`),
				)
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
				)

				runningBroker = startBroker(conf)
				Consistently(runningBroker).ShouldNot(gexec.Exit())
			})
		})

		Context("when the broker cannot obtain a UAA token for CF", func() {
			It("fails to start", func() {
				conf.CF.Authentication.UAA.ClientCredentials.Secret = "wrong-secret"
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
				)

				runningBroker = startBrokerWithoutPortCheck(conf)

				Eventually(runningBroker.Out).Should(gbytes.Say("error starting broker: The following broker startup checks failed: CF API error: Error authenticating"))
				Eventually(runningBroker).Should(gexec.Exit())
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			})
		})

		Context("when the broker cannot obtain UAA tokens for BOSH", func() {
			It("fails to start", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
					mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
				)
				conf.Bosh.Authentication.UAA.ClientCredentials.Secret = "wrong-secret"

				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
				)
				runningBroker = startBrokerWithoutPortCheck(conf)

				Eventually(runningBroker.Out).Should(gbytes.Say("error starting broker:"))
				Eventually(runningBroker.Out).Should(gbytes.Say("BOSH Director error: Failed to verify credentials"))
				Eventually(runningBroker).Should(gexec.Exit())
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			})
		})

		Context("when the broker starts and there are service instances with plans that are not present in the catalog", func() {
			It("fails to start", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
					mockcfapi.ListServiceOfferings().RespondsOKWith(`{
				  "next_url": null,
				  "resources": [
				    {
				      "entity": {
				        "unique_id": "`+serviceID+`",
				        "service_plans_url": "/v2/services/06df08f9-5a58-4d33-8097-32d0baf3ce1e/service_plans"
				      }
				    }
				  ]
				}`),
					mockcfapi.ListServicePlans("06df08f9-5a58-4d33-8097-32d0baf3ce1e").RespondsOKWith(`{
				   "next_url": null,
				   "resources": [
				      {
				         "entity": {
								 		"unique_id": "not_in_service_catalog",
				            "service_instances_url": "/v2/service_plans/ff717e7c-afd5-4d0a-bafe-16c7eff546ec/service_instances",
				                             "name": "plan_not_in_catalog"
				         }
				      }
				   ]
				}`),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(`{
				   "total_results": 1
				}`),
				)
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
				)
				conf.ServiceCatalog.Plans = []config.Plan{}

				runningBroker = startBrokerWithoutPortCheck(conf)

				Eventually(runningBroker.Out).Should(gbytes.Say(`plan plan_not_in_catalog \(not_in_service_catalog\) was expected but is now missing. You cannot remove or change the plan_id of a plan which has existing service instances`))
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
				Eventually(runningBroker).Should(gexec.Exit())
			})
		})

		Context("when the CF api version is below 2.57.0", func() {
			It("fails to start", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsOKWith(`{"api_version": "2.56.0"}`),
					mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
				)
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
				)

				runningBroker = startBrokerWithoutPortCheck(conf)

				Eventually(runningBroker.Out).Should(gbytes.Say("error starting broker: The following broker startup checks failed: CF API error: Cloud Foundry API version is insufficient, ODB requires CF v238+."))
				Eventually(runningBroker).Should(gexec.Exit())
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			})
		})

		Context("when the CF api version cannot be retrieved", func() {
			It("fails to start", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsInternalServerErrorWith("error getting info"),
					mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("I wouldn't even give you info, and you're asking for service offerings?"),
				)
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
				)

				runningBroker = startBrokerWithoutPortCheck(conf)

				Eventually(runningBroker.Out).Should(gbytes.Say(`error starting broker: The following broker startup checks failed: CF API error: Unexpected reponse status 500, "error getting info". ODB requires CF v238+.`))
				Eventually(runningBroker).Should(gexec.Exit())
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			})
		})

		Context("when disable_cf_startup_checks is set to true", func() {
			BeforeEach(func() {
				conf.Broker.DisableCFStartupChecks = true
			})

			It("does not fail and does not contact CF", func() {
				cfAPI.Close()
				cfUAA.Close()
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
				)

				runningBroker = startBroker(conf)

				odbLogPattern := `\[on-demand-service-broker\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}`

				Eventually(runningBroker.Out).Should(gbytes.Say(fmt.Sprintf(`%s Starting broker`, odbLogPattern)))
				Eventually(runningBroker.Out).Should(gbytes.Say(`-------./ssssssssssssssssssss:.-------`))
				Eventually(runningBroker.Out).Should(gbytes.Say(fmt.Sprintf(`%s Listening on :%d`, odbLogPattern, conf.Broker.Port)))
				Consistently(runningBroker).ShouldNot(gexec.Exit())
			})
		})
	})

	Describe("BOSH Director API version", func() {
		var (
			boshDirector *mockbosh.MockBOSH
			boshUAA      *mockuaa.ClientCredentialsServer
			cfAPI        *mockhttp.Server
			cfUAA        *mockuaa.ClientCredentialsServer
			conf         config.Config
		)

		BeforeEach(func() {
			boshUAA = mockuaa.NewClientCredentialsServerTLS(boshClientID, boshClientSecret, pathToSSLCerts("cert.pem"), pathToSSLCerts("key.pem"), "bosh uaa token")
			boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
			boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
			boshDirector.ExcludeAuthorizationCheck("/info")

			cfAPI = mockcfapi.New()
			cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")

			conf = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
		})

		AfterEach(func() {
			if runningBroker != nil {
				Eventually(runningBroker.Terminate()).Should(gexec.Exit())
			}
			boshDirector.VerifyMocks()
			boshDirector.Close()
			boshUAA.Close()
			cfAPI.VerifyMocks()
			cfAPI.Close()
			cfUAA.Close()
		})

		Context("with a sufficient stemcell version for ODB", func() {
			Context("and no lifecycle errands configured", func() {
				BeforeEach(func() {
					boshDirector.VerifyAndMock(
						mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
						mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					)
					cfAPI.VerifyAndMock(
						mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
						mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
					)
				})

				It("does not fail at start up", func() {
					runningBroker = startBroker(conf)
					Consistently(runningBroker).ShouldNot(gexec.Exit())
				})
			})

			Context("and lifecycle errands configured", func() {
				BeforeEach(func() {
					boshDirector.VerifyAndMock(
						mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
						mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
					)
					postDeployErrandPlan := config.Plan{
						Name: "post-deploy-errand-plan",
						ID:   "post-deploy-errand-plan-id",
						InstanceGroups: []serviceadapter.InstanceGroup{
							{
								Name:      "instance-group-name",
								VMType:    "post-deploy-errand-vm-type",
								Instances: 1,
								Networks:  []string{"net1"},
								AZs:       []string{"az1"},
							},
						},
						LifecycleErrands: &serviceadapter.LifecycleErrands{
							PostDeploy: []serviceadapter.Errand{{
								Name: "health-check",
							}},
						},
					}

					conf.ServiceCatalog.Plans = config.Plans{postDeployErrandPlan}

					cfAPI.VerifyAndMock(
						mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
						mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
					)
				})

				It("fails to start", func() {
					runningBroker = startBrokerWithoutPortCheck(conf)

					Eventually(runningBroker.Out).Should(gbytes.Say("error starting broker: The following broker startup checks failed: BOSH Director error: API version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v261+."))

					Eventually(runningBroker).Should(gexec.Exit())
					Expect(runningBroker.ExitCode()).ToNot(Equal(0))
				})
			})
		})

		Context("with a sufficient semver version for ODB", func() {
			Context("and no lifecycle errands configured", func() {
				BeforeEach(func() {
					boshDirector.VerifyAndMock(
						mockbosh.Info().RespondsWithSufficientSemverVersionForODB(boshDirector.UAAURL),
						mockbosh.Info().RespondsWithSufficientSemverVersionForODB(boshDirector.UAAURL),
					)
					cfAPI.VerifyAndMock(
						mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
						mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
					)
				})
				It("does not fail at start up", func() {
					runningBroker = startBroker(conf)
					Consistently(runningBroker).ShouldNot(gexec.Exit())
				})
			})

			Context("and lifecycle errands are configured", func() {
				BeforeEach(func() {
					postDeployErrandPlan := config.Plan{
						Name: "post-deploy-errand-plan",
						ID:   "post-deploy-errand-plan-id",
						InstanceGroups: []serviceadapter.InstanceGroup{
							{
								Name:      "instance-group-name",
								VMType:    "post-deploy-errand-vm-type",
								Instances: 1,
								Networks:  []string{"net1"},
								AZs:       []string{"az1"},
							},
						},
						LifecycleErrands: &serviceadapter.LifecycleErrands{
							PostDeploy: []serviceadapter.Errand{{
								Name: "health-check",
							}},
						},
					}

					conf.ServiceCatalog.Plans = config.Plans{postDeployErrandPlan}
					boshDirector.VerifyAndMock(
						mockbosh.Info().RespondsWithSufficientSemverVersionForODB(boshDirector.UAAURL),
						mockbosh.Info().RespondsWithSufficientSemverVersionForODB(boshDirector.UAAURL),
					)
					cfAPI.VerifyAndMock(
						mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
						mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
					)
				})

				It("fails to start", func() {
					runningBroker = startBrokerWithoutPortCheck(conf)

					Eventually(runningBroker.Out).Should(gbytes.Say("error starting broker: The following broker startup checks failed: BOSH Director error: API version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v261+."))

					Eventually(runningBroker).Should(gexec.Exit())
					Expect(runningBroker.ExitCode()).ToNot(Equal(0))
				})
			})
		})

		Context("with a sufficient semver version for lifecycle errands", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithSufficientVersionForLifecycleErrands(boshDirector.UAAURL),
					mockbosh.Info().RespondsWithSufficientVersionForLifecycleErrands(boshDirector.UAAURL),
				)
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
					mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
				)
			})

			Context("and no lifecycle errands configured", func() {
				It("does not fail at start up", func() {
					runningBroker = startBroker(conf)
					Consistently(runningBroker).ShouldNot(gexec.Exit())
				})
			})

			Context("and lifecycle errands are configured", func() {
				BeforeEach(func() {
					postDeployErrandPlan := config.Plan{
						Name: "post-deploy-errand-plan",
						ID:   "post-deploy-errand-plan-id",
						InstanceGroups: []serviceadapter.InstanceGroup{
							{
								Name:      "instance-group-name",
								VMType:    "post-deploy-errand-vm-type",
								Instances: 1,
								Networks:  []string{"net1"},
								AZs:       []string{"az1"},
							},
						},
						LifecycleErrands: &serviceadapter.LifecycleErrands{
							PostDeploy: []serviceadapter.Errand{{
								Name: "health-check",
							}},
						},
					}

					conf.ServiceCatalog.Plans = config.Plans{postDeployErrandPlan}
				})

				It("does not fail at start up", func() {
					runningBroker = startBroker(conf)
					Consistently(runningBroker).ShouldNot(gexec.Exit())
				})
			})
		})

		Context("with an insufficient stemcell version for ODB", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithVersion("1.3261.42.0 (00000000)", boshDirector.UAAURL),
					mockbosh.Info().RespondsWithVersion("1.3261.42.0 (00000000)", boshDirector.UAAURL),
				)
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
					mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
				)
			})

			It("fails to start", func() {
				runningBroker = startBrokerWithoutPortCheck(conf)

				Eventually(runningBroker.Out).Should(gbytes.Say("error starting broker: The following broker startup checks failed: BOSH Director error: API version is insufficient, ODB requires BOSH v257+."))

				Eventually(runningBroker).Should(gexec.Exit())
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			})
		})

		Context("with an unrecognised version", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithVersion("0000 (00000000)", boshDirector.UAAURL), // e.g. bosh director 260.3
					mockbosh.Info().RespondsWithVersion("0000 (00000000)", boshDirector.UAAURL), // e.g. bosh director 260.3
				)
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
					mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
				)
			})

			It("fails to start", func() {
				runningBroker = startBrokerWithoutPortCheck(conf)

				Eventually(runningBroker.Out).Should(gbytes.Say(`error starting broker: The following broker startup checks failed: BOSH Director error: unrecognised BOSH Director version: "0000 \(00000000\)". ODB requires BOSH v257+.`))

				Eventually(runningBroker).Should(gexec.Exit())
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			})
		})

		Context("when the info cannot be retrieved", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsInternalServerErrorWith("no info for you"),
				)
			})

			It("fails to start", func() {
				runningBroker = startBrokerWithoutPortCheck(conf)

				Eventually(runningBroker.Out).Should(gbytes.Say(`error fetching BOSH director information`))

				Eventually(runningBroker).Should(gexec.Exit())
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			})
		})

		Context("with insufficient BOSH and CF API versions", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Info().RespondsWithVersion("1.3261.42.0 (00000000)", boshDirector.UAAURL),
					mockbosh.Info().RespondsWithVersion("1.3261.42.0 (00000000)", boshDirector.UAAURL),
				)
				cfAPI.VerifyAndMock(
					mockcfapi.GetInfo().RespondsOKWith(`{"api_version": "2.56.0"}`),
					mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
				)
			})

			It("fails to start", func() {
				runningBroker = startBrokerWithoutPortCheck(conf)

				By("logging a CF API version error")
				Eventually(runningBroker.Out).Should(gbytes.Say("CF API error: Cloud Foundry API version is insufficient, ODB requires CF v238+"))

				By("logging a BOSH Director API version error")
				Eventually(runningBroker.Out).Should(gbytes.Say("BOSH Director error: API version is insufficient, ODB requires BOSH v257+."))

				Eventually(runningBroker).Should(gexec.Exit())
				Expect(runningBroker.ExitCode()).ToNot(Equal(0))
			})
		})
	})
})
