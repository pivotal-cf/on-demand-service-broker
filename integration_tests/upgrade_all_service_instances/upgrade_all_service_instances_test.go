// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_all_service_instances_test

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	. "github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

const (
	brokerUsername = "broker username"
	brokerPassword = "broker password"
)

var _ = Describe("running the tool to upgrade all service instances", func() {
	startUpgradeAllInstanceBinary := func(errandConfig config.InstanceIteratorConfig) *gexec.Session {
		b, err := yaml.Marshal(errandConfig)
		Expect(err).ToNot(HaveOccurred())
		configPath := writeConfigFile(string(b))
		return StartBinaryWithParams(binaryPath, []string{"-configPath", configPath})
	}

	Describe("upgrading only via BOSH", func() {
		var broker *ghttp.Server

		When("the broker is not configured with TLS", func() {
			var (
				serviceInstances string
				instanceID       string

				lastOperationHandler    *FakeHandler
				serviceInstancesHandler *FakeHandler
				upgradeHandler          *FakeHandler
				errandConfig            config.InstanceIteratorConfig
			)

			BeforeEach(func() {
				broker = ghttp.NewServer()
				errandConfig = errandConfigurationBOSH(broker.URL())

				serviceInstancesHandler, instanceID, serviceInstances = handleServiceInstanceList(broker)
				upgradeHandler = handleBOSHServiceInstanceUpgrade(broker)
				lastOperationHandler = handleBOSHLastOperation(broker)
			})

			AfterEach(func() {
				broker.Close()
			})

			It("exits successfully and upgrades the instance", func() {
				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
				Expect(runningTool).To(SatisfyAll(
					Not(gbytes.Say("Upgrading all instances via CF")),
					gbytes.Say("Upgrading all instances via BOSH"),
					gbytes.Say("Sleep interval until next attempt: 2s"),
					gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
					gbytes.Say("Number of successful operations: 1"),
				))
				Expect(upgradeHandler.GetRequestForCall(0).Body).To(MatchJSON(`{
					"plan_id": "service-plan-id",
					"context": {"space_guid": "the-space-guid"}
				}`))
			})

			It("exits successfully when all instances are already up-to-date", func() {
				runningTool := startUpgradeAllInstanceBinary(errandConfig)
				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
				Expect(runningTool).To(gbytes.Say("Number of successful operations: 1"))

				By("running upgrade all again")
				upgradeHandler.RespondsWith(http.StatusNoContent, "")
				runningTool = startUpgradeAllInstanceBinary(errandConfig)
				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))

				Expect(runningTool).To(SatisfyAll(
					gbytes.Say(`Result: instance already up to date - operation skipped`),
					gbytes.Say("Sleep interval until next attempt: 2s"),
					gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
					gbytes.Say("Number of skipped operations: 1"),
				))
			})

			It("uses the canary_selection_params when querying canary instances", func() {
				instanceID := "my-instance-id"
				canaryInstanceID := "canary-instance-id"
				canariesList := fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, canaryInstanceID)
				serviceInstances := fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}, {"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID, canaryInstanceID)

				serviceInstancesHandler.WithQueryParams().RespondsWith(http.StatusOK, serviceInstances)
				serviceInstancesHandler.WithQueryParams("foo=bar").RespondsWith(http.StatusOK, canariesList)
				lastOperationHandler.RespondsWith(http.StatusOK, `{"state":"succeeded"}`)

				errandConfig.CanarySelectionParams = map[string]string{"foo": "bar"}
				errandConfig.Canaries = 1

				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
				Expect(runningTool).To(SatisfyAll(
					gbytes.Say(`\[upgrade\-all\] STARTING CANARIES: 1 canaries`),
					gbytes.Say(`\[canary-instance-id] Starting to process service instance`),
					gbytes.Say(`\[upgrade\-all\] FINISHED CANARIES`),
					gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
					gbytes.Say("Number of successful operations: 2"),
				))
			})

			It("uses the canary_selection_params but returns an error if no instances found but instances exist", func() {
				canariesList := `[]`

				serviceInstancesHandler.WithQueryParams("cf_org=my-org", "cf_space=my-space").RespondsWith(http.StatusOK, canariesList)

				errandConfig.CanarySelectionParams = map[string]string{"cf_org": "my-org", "cf_space": "my-space"}
				errandConfig.Canaries = 1

				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
				Expect(runningTool).To(gbytes.Say("Failed to find a match to the canary selection criteria"))
			})

			It("returns an error if service-instances api responds with a non-200", func() {
				serviceInstancesHandler.RespondsWith(http.StatusInternalServerError, `{"description": "a forced error"}`)

				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
				Expect(runningTool).To(gbytes.Say("error listing service instances"))
				Expect(runningTool).To(gbytes.Say("500"))
			})

			It("exits with a failure and shows a summary message when the upgrade fails", func() {
				lastOperationHandler.RespondsOnCall(1, http.StatusOK, `{"state":"failed"}`)

				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
				Expect(runningTool).To(gbytes.Say("Status: FAILED"))
				Expect(runningTool).To(gbytes.Say(fmt.Sprintf(`Number of service instances that failed to process: 1 \[%s\]`, instanceID)))
			})

			When("the attempt limit is reached", func() {
				It("exits with an error reporting the instances that were not upgraded", func() {
					upgradeHandler.RespondsWith(http.StatusConflict, "")

					runningTool := startUpgradeAllInstanceBinary(errandConfig)

					Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
					Expect(runningTool).To(SatisfyAll(
						gbytes.Say(`\[upgrade\-all\] Processing all instances. Attempt 1/2`),
						gbytes.Say(`\[upgrade\-all\] Processing all remaining instances. Attempt 2/2`),
						gbytes.Say("Number of busy instances which could not be processed: 1"),
						gbytes.Say(fmt.Sprintf("The following instances could not be processed: %s", instanceID)),
					))
				})
			})

			When("a service instance plan is updated after upgrade-all starts but before instance upgrade", func() {
				It("uses the new plan for the upgrade", func() {
					spaceGuid := "some-space-guid"
					serviceInstancesInitialResponse := fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s", "space_guid": "%s"}]`, instanceID, spaceGuid)
					serviceInstancesResponseAfterPlanUpdate := fmt.Sprintf(`[{"plan_id": "service-plan-id-2", "service_instance_id": "%s", "space_guid": "%s"}]`, instanceID, spaceGuid)

					serviceInstancesHandler.RespondsOnCall(0, http.StatusOK, serviceInstancesInitialResponse)
					serviceInstancesHandler.RespondsOnCall(1, http.StatusOK, serviceInstancesResponseAfterPlanUpdate)

					runningTool := startUpgradeAllInstanceBinary(errandConfig)

					Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
					Expect(runningTool).To(SatisfyAll(
						gbytes.Say("Sleep interval until next attempt: 2s"),
						gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
						gbytes.Say("Number of successful operations: 1"),
					))

					Expect(upgradeHandler.GetRequestForCall(0).Body).To(MatchJSON(fmt.Sprintf(`{
						"plan_id": "service-plan-id-2",
						"context": {"space_guid": %q}
					}`, spaceGuid)))
				})
			})

			When("a service instance is deleted after upgrade-all starts but before the instance upgrade", func() {
				It("Fetches the latest service instances info and reports a deleted service", func() {
					serviceInstancesHandler.RespondsOnCall(0, http.StatusOK, serviceInstances)
					serviceInstancesHandler.RespondsOnCall(1, http.StatusOK, "[]")

					runningTool := startUpgradeAllInstanceBinary(errandConfig)

					Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
					Expect(runningTool).To(SatisfyAll(
						gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
						gbytes.Say("Number of successful operations: 0"),
						gbytes.Say("Number of deleted instances before operation could happen: 1"),
					))
				})
			})

			When("a service instance refresh fails prior to instance upgrade", func() {
				It("logs failure and carries on with previous data", func() {
					serviceInstancesHandler.RespondsOnCall(0, http.StatusOK, serviceInstances)
					serviceInstancesHandler.RespondsOnCall(1, http.StatusInternalServerError, "oops")

					runningTool := startUpgradeAllInstanceBinary(errandConfig)

					Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
					Expect(runningTool).To(SatisfyAll(
						gbytes.Say("Failed to get refreshed list of instances. Continuing with previously fetched info"),
						gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
						gbytes.Say("Number of successful operations: 1"),
					))
				})
			})

			When("a single upgrade fails in the middle of multiple instance upgrades", func() {
				var runningTool *gexec.Session

				BeforeEach(func() {
					serviceInstanceList := `[{"plan_id": "service-plan-id", "service_instance_id": "first-instance"},
						{"plan_id": "service-plan-id", "service_instance_id": "second-instance"},
						{"plan_id": "service-plan-id", "service_instance_id": "third-instance"},
						{"plan_id": "service-plan-id", "service_instance_id": "fourth-instance"}]`
					serviceInstancesHandler.WithQueryParams().RespondsWith(http.StatusOK, serviceInstanceList)

					lastOperationHandler.RespondsOnCall(0, http.StatusOK, `{"state":"in progress"}`)
					lastOperationHandler.RespondsOnCall(1, http.StatusOK, `{"state":"succeeded"}`)
					lastOperationHandler.RespondsOnCall(2, http.StatusOK, `{"state":"in progress"}`)
					lastOperationHandler.RespondsOnCall(3, http.StatusOK, `{"state":"succeeded"}`)
					lastOperationHandler.RespondsOnCall(4, http.StatusOK, `{"state":"in progress"}`)
					lastOperationHandler.RespondsOnCall(5, http.StatusOK, `{"state":"failed"}`) // FAIL
					lastOperationHandler.RespondsOnCall(6, http.StatusOK, `{"state":"in progress"}`)
					lastOperationHandler.RespondsOnCall(7, http.StatusOK, `{"state":"succeeded"}`)
				})

				It("upgrades instances beyond the point of failure while still returning an error", func() {
					runningTool = startUpgradeAllInstanceBinary(errandConfig)

					By("returning overall errand failure")
					Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))

					By("attempting to upgrade all instances and continuing in spite of failure")
					Expect(runningTool).To(SatisfyAll(
						gbytes.Say(`Service Instances: first\-instance second\-instance third\-instance`),
						gbytes.Say(`\[first\-instance\] Starting to process`),
						gbytes.Say(`\[first\-instance\] Result: Service Instance operation success`),
						gbytes.Say(`\[second\-instance\] Starting to process`),
						gbytes.Say(`\[second\-instance\] Result: Service Instance operation success`),
						gbytes.Say(`\[third\-instance\] Starting to process`),
						gbytes.Say(`\[third\-instance\] Result: Service Instance operation failure`),
						gbytes.Say(`\[fourth\-instance\] Starting to process`),
						gbytes.Say(`\[fourth\-instance\] Result: Service Instance operation success`)))

					By("reporting accurate success and failure counts")
					Expect(runningTool).To(SatisfyAll(
						gbytes.Say(`PROCESSING Status: FAILED; Summary: Number of successful operations: 3;`),
						gbytes.Say(`Number of skipped operations: 0;`),
						gbytes.Say(`Number of service instances that failed to process: 1 \[third\-instance\]`)))
				})
			})
		})

		When("the broker is configured with TLS", func() {
			var (
				pemCert      string
				errandConfig config.InstanceIteratorConfig
			)

			BeforeEach(func() {
				broker = ghttp.NewTLSServer()
				broker.HTTPTestServer.Config.ErrorLog = loggerfactory.New(GinkgoWriter, "server", loggerfactory.Flags).New()
				rawPem := broker.HTTPTestServer.Certificate().Raw
				pemCert = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rawPem}))

				errandConfig = errandConfigurationBOSH(broker.URL())

				handleServiceInstanceList(broker)
				handleBOSHServiceInstanceUpgrade(broker)
				handleBOSHLastOperation(broker)
			})

			AfterEach(func() {
				broker.Close()
			})

			It("upgrades all instances", func() {
				errandConfig.BrokerAPI.TLS.CACert = pemCert

				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
				Expect(runningTool).To(gbytes.Say("Number of successful operations: 1"))
			})

			It("skips ssl cert verification when disabled", func() {
				errandConfig.BrokerAPI.TLS.DisableSSLCertVerification = true

				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
				Expect(runningTool).To(gbytes.Say("Number of successful operations: 1"))
			})
		})
	})

	Describe("upgrading via CF and BOSH", func() {
		var (
			broker       *ghttp.Server
			cfApi        *ghttp.Server
			uaaApi       *ghttp.Server
			errandConfig config.InstanceIteratorConfig
		)

		BeforeEach(func() {
			broker = ghttp.NewServer()
			cfApi = ghttp.NewServer()
			uaaApi = ghttp.NewServer()

			errandConfig = errandConfigurationCF(broker.URL(), cfApi.URL(), uaaApi.URL())

			handleUAA(uaaApi)
			handleServiceInstanceList(broker)

			handleBOSHServiceInstanceUpgrade(broker)
			handleBOSHLastOperation(broker)

			handleCFInfo(cfApi)
			handleCFServicePlans(cfApi)
		})

		AfterEach(func() {
			broker.Close()
			cfApi.Close()
			uaaApi.Close()
		})

		When("an upgrade is available via CF", func() {
			BeforeEach(func() {
				cfApi.RouteToHandler(http.MethodPut, regexp.MustCompile(`/v2/service_instances/.*`),
					ghttp.CombineHandlers(
						ghttp.RespondWith(http.StatusAccepted, `{"entity": {"last_operation": { "type": "update", "state": "in progress" }}}`),
					),
				)

				cfApi.RouteToHandler(http.MethodGet, regexp.MustCompile(`/v2/service_instances/.*`),
					ghttp.CombineHandlers(
						ghttp.RespondWith(http.StatusOK, `{"entity": {"last_operation": { "type": "update", "state": "succeeded" }}}`),
					),
				)
			})

			It("upgrades via CF then BOSH", func() {
				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
				Expect(runningTool).To(SatisfyAll(
					gbytes.Say("Upgrading all instances via CF"),
					gbytes.Say("Sleep interval until next attempt: 2s"),
					gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
					gbytes.Say("Number of successful operations: 1"),
					gbytes.Say("Number of skipped operations: 0"),
					gbytes.Say("Upgrading all instances via BOSH"),
					gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
					gbytes.Say("Number of successful operations: 1"),
					gbytes.Say("Number of skipped operations: 0"),
				))
			})

			When("the CF upgrade fails", func() {
				It("doesn't do BOSH upgrades", func() {
					cfApi.RouteToHandler(http.MethodGet, regexp.MustCompile(`/v2/service_instances/.*`),
						ghttp.CombineHandlers(
							ghttp.RespondWith(http.StatusOK, `{"entity": {"last_operation": { "type": "update", "state": "failed" }}}`),
						),
					)

					runningTool := startUpgradeAllInstanceBinary(errandConfig)

					Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
					Expect(runningTool).To(SatisfyAll(
						gbytes.Say("Upgrading all instances via CF"),
						gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: FAILED`),
						gbytes.Say("Number of service instances that failed to process: 1"),
						Not(gbytes.Say("Upgrading all instances via BOSH")),
					))
				})
			})
		})

		When("no upgrades are available via CF", func() {
			BeforeEach(func() {
				cfApi.RouteToHandler(http.MethodPut, regexp.MustCompile(`/v2/service_instances/.*`),
					ghttp.CombineHandlers(
						ghttp.RespondWith(http.StatusCreated, `{"entity": {"last_operation": {"type": "update", "state": "succeeded"}}}`),
					),
				)
			})

			It("says that the CF upgrade was skipped, and does the upgrade via BOSH", func() {
				runningTool := startUpgradeAllInstanceBinary(errandConfig)

				Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
				Expect(runningTool).To(SatisfyAll(
					gbytes.Say("Upgrading all instances via CF"),
					gbytes.Say("instance already up to date - operation skipped"),
					gbytes.Say("Sleep interval until next attempt: 2s"),
					gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
					gbytes.Say("Number of successful operations: 0"),
					gbytes.Say("Number of skipped operations: 1"),
					gbytes.Say("Upgrading all instances via BOSH"),
					gbytes.Say(`\[upgrade\-all\] FINISHED PROCESSING Status: SUCCESS`),
					gbytes.Say("Number of successful operations: 1"),
					gbytes.Say("Number of skipped operations: 0"),
				))
			})
		})
	})
})

func writeConfigFile(configContent string) string {
	file, err := ioutil.TempFile("", "config")
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	_, err = file.Write([]byte(configContent))
	Expect(err).NotTo(HaveOccurred())

	return file.Name()
}

func handleServiceInstanceList(broker *ghttp.Server) (*FakeHandler, string, string) {
	serviceInstancesHandler := new(FakeHandler)

	broker.RouteToHandler(http.MethodGet, "/mgmt/service_instances", ghttp.CombineHandlers(
		ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
		serviceInstancesHandler.Handle,
	))
	instanceID := "service-instance-id"
	response := `[{"plan_id": "service-plan-id", "service_instance_id": "%s", "space_guid": "the-space-guid"}]`
	serviceInstances := fmt.Sprintf(response, instanceID)
	serviceInstancesHandler.RespondsWith(http.StatusOK, serviceInstances)

	return serviceInstancesHandler, instanceID, serviceInstances
}

func handleBOSHServiceInstanceUpgrade(broker *ghttp.Server) *FakeHandler {
	upgradeHandler := new(FakeHandler)

	broker.RouteToHandler(http.MethodPatch, regexp.MustCompile(`/mgmt/service_instances/.*`), ghttp.CombineHandlers(
		ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
		ghttp.VerifyRequest(http.MethodPatch, ContainSubstring("/mgmt/service_instances/"), "operation_type=upgrade"),
		upgradeHandler.Handle,
	))
	operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{},"PreDeleteErrand":{}}`

	upgradeHandler.RespondsWith(http.StatusAccepted, operationData)

	return upgradeHandler
}

func handleBOSHLastOperation(broker *ghttp.Server) *FakeHandler {
	lastOperationHandler := new(FakeHandler)

	broker.RouteToHandler(http.MethodGet, regexp.MustCompile(`/v2/service_instances/.*/last_operation`), ghttp.CombineHandlers(
		ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
		lastOperationHandler.Handle,
	))

	lastOperationHandler.RespondsOnCall(0, http.StatusOK, `{"state":"in progress"}`)
	lastOperationHandler.RespondsOnCall(1, http.StatusOK, `{"state":"succeeded"}`)

	return lastOperationHandler
}

func handleUAA(uaaAPI *ghttp.Server) {
	uaaAuthenticationHandler := new(FakeHandler)
	uaaAPI.RouteToHandler(http.MethodPost, regexp.MustCompile(`/oauth/token`), ghttp.CombineHandlers(
		uaaAuthenticationHandler.Handle,
	))
	authenticationResponse := `{ "access_token": "some-random-token", "expires_in": 3600}`
	uaaAuthenticationHandler.RespondsWith(http.StatusOK, authenticationResponse)
}

func handleCFInfo(cfAPI *ghttp.Server) {
	cfInfoHandler := new(FakeHandler)

	cfAPI.RouteToHandler(http.MethodGet, "/v2/info", ghttp.CombineHandlers(
		cfInfoHandler.Handle))

	cfInfoResponse := `{"api_version": "2.139.0","osbapi_version": "2.15"}`
	cfInfoHandler.RespondsWith(http.StatusOK, cfInfoResponse)
}

func handleCFServicePlans(cfAPI *ghttp.Server) {
	servicePlanHandler := new(FakeHandler)
	cfAPI.RouteToHandler(http.MethodGet, regexp.MustCompile(`/v2/service_plans`), ghttp.CombineHandlers(
		servicePlanHandler.Handle,
	))
	servicePlanResponse := `{ "resources":[{ "entity": { "maintenance_info": { "version": "0.31.0" }}}]}`
	servicePlanHandler.RespondsWith(http.StatusOK, servicePlanResponse)
}

func errandConfigurationBOSH(brokerURL string) config.InstanceIteratorConfig {
	return config.InstanceIteratorConfig{
		PollingInterval: 1,
		AttemptLimit:    2,
		AttemptInterval: 2,
		MaxInFlight:     1,
		BrokerAPI: config.BrokerAPI{
			URL: brokerURL,
			Authentication: config.Authentication{
				Basic: config.UserCredentials{
					Username: brokerUsername,
					Password: brokerPassword,
				},
			},
		},
	}
}

func errandConfigurationCF(brokerURL, cfURL, uaaURL string) config.InstanceIteratorConfig {
	errandConfig := errandConfigurationBOSH(brokerURL)
	errandConfig.CF = config.CF{
		URL: cfURL,
		UAA: config.UAAConfig{
			URL: uaaURL,
			Authentication: config.UAACredentials{
				UserCredentials: config.UserCredentials{
					Username: "cf-username",
					Password: "cf-password",
				},
			},
		},
		DisableSSLCertVerification: true,
	}
	errandConfig.MaintenanceInfoPresent = true
	return errandConfig
}
