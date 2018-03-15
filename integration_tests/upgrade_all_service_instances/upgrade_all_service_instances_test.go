// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_all_service_instances_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbroker"
)

func pathToSSLCerts(filename string) string {
	return fmt.Sprintf("../fixtures/ssl/%s", filename)
}

func populateBrokerConfig(odbURL, brokerUsername, brokerPassword string) string {
	return fmt.Sprintf(`---
broker_api:
  url: %s
  authentication:
    basic:
      username: %s
      password: %s`, odbURL, brokerUsername, brokerPassword)
}

func populateServiceInstancesAPIConfig(
	serviceInstancesAPIURLPath,
	serviceInstancesAPIUsername,
	serviceInstancesAPIPassword string) string {
	return fmt.Sprintf(`

service_instances_api:
  url: %s
  authentication:
    basic:
      username: %s
      password: %s`, serviceInstancesAPIURLPath, serviceInstancesAPIUsername, serviceInstancesAPIPassword)
}

func populateServiceInstancesAPISSLConfig(
	serviceInstancesAPIURLPath,
	serviceInstancesAPIUsername,
	serviceInstancesAPIPassword,
	serviceInstancesAPIRootCA string) string {

	formattedCert := strings.Replace(serviceInstancesAPIRootCA, "\n", "\n    ", -1)
	return fmt.Sprintf(`
service_instances_api:
  url: %s
  root_ca_cert: |
    %s
  authentication:
    basic:
      username: %s
      password: %s`,
		serviceInstancesAPIURLPath,
		formattedCert,
		serviceInstancesAPIUsername,
		serviceInstancesAPIPassword,
	)
}

func populateUpgraderConfig(pollingInterval, attemptInterval, attemptLimit int) string {
	return fmt.Sprintf(`
polling_interval: %d
attempt_interval: %d
attempt_limit: %d
max_in_flight: 1`, pollingInterval, attemptInterval, attemptLimit)
}

func populateUpgraderConfigWithCanaries(canaries int, org, space string) string {
	return fmt.Sprintf(`
polling_interval: 1
attempt_interval: 2
attempt_limit: 5
max_in_flight: 1
canaries: %d
canary_selection_params:
  cf_org: %s
  cf_space: %s`, canaries, org, space)
}

func writeConfigFile(configContent string) string {
	file, err := ioutil.TempFile("", "config")
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	_, err = file.Write([]byte(configContent))
	Expect(err).NotTo(HaveOccurred())

	return file.Name()
}

func startNewAPIServer(serviceInstancesAPIURLPath, serviceInstancesAPIUsername, serviceInstancesAPIPassword string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, _ := r.BasicAuth()
		if username != serviceInstancesAPIUsername || password != serviceInstancesAPIPassword {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != serviceInstancesAPIURLPath {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		fmt.Fprintln(w, `[{"service_instance_id": "service-instance-id", "plan_id": "service-plan-id"}]`)
	}))
}

func startNewSSLAPIServer(
	certPath,
	keyPath,
	serviceInstancesAPIURLPath,
	serviceInstancesAPIUsername,
	serviceInstancesAPIPassword string) *httptest.Server {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, _ := r.BasicAuth()
		if username != serviceInstancesAPIUsername || password != serviceInstancesAPIPassword {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != serviceInstancesAPIURLPath {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		fmt.Fprintln(w, `[{"service_instance_id": "service-instance-id", "plan_id": "service-plan-id"}]`)
	})
	cer, err := tls.LoadX509KeyPair(certPath, keyPath)
	Expect(err).NotTo(HaveOccurred())
	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	sslServer := httptest.NewUnstartedServer(handler)
	sslServer.TLS = config

	sslServer.Config.ErrorLog = loggerfactory.New(GinkgoWriter, "server", loggerfactory.Flags).New()
	sslServer.StartTLS()
	return sslServer
}

var _ = Describe("running the tool to upgrade all service instances", func() {
	const (
		brokerUsername                = "broker username"
		brokerPassword                = "broker password"
		brokerServiceInstancesURLPath = "/mgmt/service_instances"
		serviceInstancesAPIUsername   = "siapi username"
		serviceInstancesAPIPassword   = "siapi password"
		serviceInstancesAPIURLPath    = "/some-service-instances-come-from-here"
	)

	var (
		odb        *mockhttp.Server
		configPath string
		certPath   string
		keyPath    string
	)

	startUpgradeAllInstanceBinary := func() *gexec.Session {
		return helpers.StartBinaryWithParams(binaryPath, []string{"-configPath", configPath})
	}

	BeforeEach(func() {
		certPath = pathToSSLCerts("cert.pem")
		keyPath = pathToSSLCerts("key.pem")

		odb = mockbroker.New()
		odb.ExpectedBasicAuth(brokerUsername, brokerPassword)
	})

	AfterEach(func() {
		odb.VerifyMocks()
		odb.Close()
		err := os.Remove(configPath)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when service-instances-api is specified in the config", func() {
		var testServiceInstancesAPIServer *httptest.Server

		AfterEach(func() {
			testServiceInstancesAPIServer.Close()
		})

		It("exits successfully with one instance upgraded message", func() {
			testServiceInstancesAPIServer = startNewAPIServer(
				serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword)

			operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{},"PreDeleteErrand":{}}`
			instanceID := "service-instance-id"
			odb.VerifyAndMock(
				mockbroker.UpgradeInstance(instanceID).RespondsAcceptedWith(operationData),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationInProgress(),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationSucceeded(),
			)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				testServiceInstancesAPIServer.URL+serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword,
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
			Expect(runningTool).To(gbytes.Say("Sleep interval until next attempt: 2s"))
			Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 1"))
		})

		It("returns unauthorised when incorrect service instances API username provided", func() {
			testServiceInstancesAPIServer = startNewAPIServer(
				serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				testServiceInstancesAPIServer.URL+serviceInstancesAPIURLPath,
				"not-the-user",
				serviceInstancesAPIPassword,
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say(fmt.Sprintf(
				`error listing service instances: error communicating with service_instances_api \(%s\): HTTP response status: 401 Unauthorized`,
				testServiceInstancesAPIServer.URL+serviceInstancesAPIURLPath,
			)))
		})

		It("returns service instances API error when URL is invalid", func() {
			testServiceInstancesAPIServer = startNewAPIServer(
				serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				"http://not-a-url",
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword,
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say(
				`error listing service instances: error communicating with service_instances_api \(http://not-a-url\):`,
			))
		})

		It("exits successfully when configured with a TLS enabled service-instances-api server", func() {
			testServiceInstancesAPIServer = startNewSSLAPIServer(
				certPath,
				keyPath,
				serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword)

			operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{},"PreDeleteErrand":{}}`
			instanceID := "service-instance-id"
			odb.VerifyAndMock(
				mockbroker.UpgradeInstance(instanceID).RespondsAcceptedWith(operationData),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationInProgress(),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationSucceeded(),
			)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIRootCA, err := ioutil.ReadFile(certPath)
			Expect(err).NotTo(HaveOccurred())
			serviceInstancesAPIConfig := populateServiceInstancesAPISSLConfig(
				testServiceInstancesAPIServer.URL+serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword,
				string(serviceInstancesAPIRootCA),
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
			Expect(runningTool).To(gbytes.Say("Sleep interval until next attempt: 2s"))
			Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 1"))
		})

		It("exits 1 when SIAPI server has TLS enabled but root CA has not been provided to errand", func() {
			testServiceInstancesAPIServer = startNewSSLAPIServer(
				certPath,
				keyPath,
				serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)

			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				testServiceInstancesAPIServer.URL+serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername,
				serviceInstancesAPIPassword)

			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)

			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig

			configPath = writeConfigFile(config)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say(fmt.Sprintf(
				"SSL validation error for `service_instances_api.url`: %s. Please configure a `service_instances_api.root_ca_cert` and use a valid SSL certificate",
				testServiceInstancesAPIServer.URL+serviceInstancesAPIURLPath)))
		})
	})

	It("uses the canary_selection_params when querying canary instances", func() {
		operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{},"PreDeleteErrand":{}}`
		instanceID := "my-instance-id"
		canaryInstanceID := "canary-instance-id"
		canariesList := fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, canaryInstanceID)
		instancesList := fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}, {"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, canaryInstanceID, instanceID)
		odb.VerifyAndMock(
			mockbroker.ListInstances().RespondsOKWith(instancesList),
			mockbroker.ListInstancesWithOrgAndSpace("my-org", "my-space").RespondsOKWith(canariesList),
			mockbroker.ListInstances().RespondsOKWith(instancesList),
			mockbroker.UpgradeInstance(canaryInstanceID).RespondsAcceptedWith(operationData),
			mockbroker.LastOperation(canaryInstanceID, operationData).RespondWithOperationSucceeded(),

			mockbroker.ListInstances().RespondsOKWith(instancesList),
			mockbroker.UpgradeInstance(instanceID).RespondsAcceptedWith(operationData),
			mockbroker.LastOperation(instanceID, operationData).RespondWithOperationSucceeded(),
		)
		brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
		serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
			odb.URL+brokerServiceInstancesURLPath,
			brokerUsername,
			brokerPassword,
		)
		pollingIntervalConfig := populateUpgraderConfigWithCanaries(1, "my-org", "my-space")
		config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
		configPath = writeConfigFile(config)

		runningTool := startUpgradeAllInstanceBinary()
		Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
		Expect(runningTool).To(gbytes.Say("STARTING CANARY UPGRADES: 1 canaries"))
		Expect(runningTool).To(gbytes.Say("Status: SUCCESS"))
		Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 2"))
	})

	It("uses the canary_selection_params but returns an error if no instances found but instances exist", func() {
		instanceID := "my-instance-id"
		canariesList := `[]`
		instancesList := fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)
		odb.VerifyAndMock(
			mockbroker.ListInstances().RespondsOKWith(instancesList),
			mockbroker.ListInstancesWithOrgAndSpace("my-org", "my-space").RespondsOKWith(canariesList),
		)
		brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
		serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
			odb.URL+brokerServiceInstancesURLPath,
			brokerUsername,
			brokerPassword,
		)
		pollingIntervalConfig := populateUpgraderConfigWithCanaries(1, "my-org", "my-space")
		config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
		configPath = writeConfigFile(config)

		runningTool := startUpgradeAllInstanceBinary()
		Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
		Expect(runningTool).To(gbytes.Say("error listing service instances: Upgrade failed to find a match to the canary selection criteria"))
	})

	It("returns an error if service-instances api responds with a non-200", func() {
		odb.VerifyAndMock(
			mockbroker.ListInstances().RespondsInternalServerErrorWith(`{"description": "a forced error"}`),
		)
		brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
		serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
			odb.URL+brokerServiceInstancesURLPath,
			brokerUsername,
			brokerPassword,
		)
		pollingIntervalConfig := populateUpgraderConfigWithCanaries(1, "my-org", "my-space")
		config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
		configPath = writeConfigFile(config)

		runningTool := startUpgradeAllInstanceBinary()
		Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
		Expect(runningTool).To(gbytes.Say("error listing service instances: HTTP response status: 500 Internal Server Error. a forced error"))
	})

	It("when there is one service instance exits successfully with one instance upgraded message", func() {
		operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{},"PreDeleteErrand":{}}`
		instanceID := "service-instance-id"
		odb.VerifyAndMock(
			mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
			mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
			mockbroker.UpgradeInstance(instanceID).RespondsAcceptedWith(operationData),
			mockbroker.LastOperation(instanceID, operationData).RespondWithOperationInProgress(),
			mockbroker.LastOperation(instanceID, operationData).RespondWithOperationSucceeded(),
		)

		brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
		serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
			odb.URL+brokerServiceInstancesURLPath,
			brokerUsername,
			brokerPassword,
		)
		pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
		config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
		configPath = writeConfigFile(config)

		runningTool := startUpgradeAllInstanceBinary()

		Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
		Expect(runningTool).To(gbytes.Say("Sleep interval until next attempt: 2s"))
		Expect(runningTool).To(gbytes.Say("Status: SUCCESS"))
		Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 1"))
	})

	It("when there is one service instance which fails to upgrade, exits with failure and shows summary message", func() {
		operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{},"PreDeleteErrand":{}}`
		instanceID := "service-instance-id"
		odb.VerifyAndMock(
			mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
			mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
			mockbroker.UpgradeInstance(instanceID).RespondsAcceptedWith(operationData),
			mockbroker.LastOperation(instanceID, operationData).RespondWithOperationInProgress(),
			mockbroker.LastOperation(instanceID, operationData).RespondWithOperationFailed(),
		)

		brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
		serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
			odb.URL+brokerServiceInstancesURLPath,
			brokerUsername,
			brokerPassword,
		)
		pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
		config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
		configPath = writeConfigFile(config)

		runningTool := startUpgradeAllInstanceBinary()

		Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
		Expect(runningTool).To(gbytes.Say("Status: FAILED"))
		Expect(runningTool).To(gbytes.Say(fmt.Sprintf(`Number of service instances that failed to upgrade: 1 \[%s\]`, instanceID)))
	})

	Context("when the attempt limit is reached", func() {
		It("exits with an error reporting the instances that were not upgraded", func() {
			instanceID := "service-instance-id"
			odb.VerifyAndMock(
				mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
				mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
				mockbroker.UpgradeInstance(instanceID).RespondsConflict(),
				mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
				mockbroker.UpgradeInstance(instanceID).RespondsConflict(),
			)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				odb.URL+brokerServiceInstancesURLPath,
				brokerUsername,
				brokerPassword,
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 2)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("Upgrading all instances. Attempt 1/2"))
			Expect(runningTool).To(gbytes.Say("Upgrading all remaining instances. Attempt 2/2"))
			Expect(runningTool).To(gbytes.Say("Number of busy instances which could not be upgraded: 1"))
			Expect(runningTool).To(gbytes.Say(fmt.Sprintf("The following instances could not be upgraded: service-instance_%s", instanceID)))
		})
	})

	Context("when the upgrade errors", func() {
		It("exits non-zero with the error message", func() {
			odb.VerifyAndMock(
				mockbroker.ListInstances().RespondsUnauthorizedWith(""),
			)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				odb.URL+brokerServiceInstancesURLPath,
				brokerUsername,
				brokerPassword,
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("error listing service instances: HTTP response status: 401 Unauthorized"))
		})
	})

	Context("when a service instance plan is updated post upgrade-all start but before instance upgrade", func() {
		It("uses the new plan for the upgrade", func() {
			operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{},"PreDeleteErrand":{}}`
			instanceID := "service-instance-id"
			odb.VerifyAndMock(
				mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
				mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id-2", "service_instance_id": "%s"}]`, instanceID)),
				mockbroker.UpgradeInstance(instanceID).
					WithBody(`{"plan_id": "service-plan-id-2"}`).
					RespondsAcceptedWith(operationData),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationInProgress(),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationSucceeded(),
			)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				odb.URL+brokerServiceInstancesURLPath,
				brokerUsername,
				brokerPassword,
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
			Expect(runningTool).To(gbytes.Say("Sleep interval until next attempt: 2s"))
			Expect(runningTool).To(gbytes.Say("Status: SUCCESS"))
			Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 1"))
		})
	})

	Context("when a service instance is deleted post the upgrade-all start but before the instance upgrade", func() {
		It("Fetches the latest service instances info and reports a deleted service", func() {
			instanceID := "service-instance-id"
			odb.VerifyAndMock(
				mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
				mockbroker.ListInstances().RespondsOKWith("[]"),
			)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				odb.URL+brokerServiceInstancesURLPath,
				brokerUsername,
				brokerPassword,
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
			Expect(runningTool).To(gbytes.Say("Status: SUCCESS"))
			Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 0"))
			Expect(runningTool).To(gbytes.Say("Number of deleted instances before upgrade could occur: 1"))
		})
	})

	Context("when a service instance refresh fails prior to instance upgrade", func() {
		It("we log failure and carry on with previous data", func() {
			operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{},"PreDeleteErrand":{}}`
			instanceID := "service-instance-id"
			odb.VerifyAndMock(
				mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
				mockbroker.ListInstances().RespondsInternalServerErrorWith("oops"),
				mockbroker.UpgradeInstance(instanceID).
					WithBody(`{"plan_id": "service-plan-id"}`).
					RespondsAcceptedWith(operationData),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationInProgress(),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationSucceeded(),
			)

			brokerConfig := populateBrokerConfig(odb.URL, brokerUsername, brokerPassword)
			serviceInstancesAPIConfig := populateServiceInstancesAPIConfig(
				odb.URL+brokerServiceInstancesURLPath,
				brokerUsername,
				brokerPassword,
			)
			pollingIntervalConfig := populateUpgraderConfig(1, 2, 5)
			config := brokerConfig + serviceInstancesAPIConfig + pollingIntervalConfig
			configPath = writeConfigFile(config)

			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))

			Expect(runningTool).To(gbytes.Say("Failed to get refreshed list of instances. Continuing with previously fetched info"))
			Expect(runningTool).To(gbytes.Say("Status: SUCCESS"))
			Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 1"))
		})
	})
})
