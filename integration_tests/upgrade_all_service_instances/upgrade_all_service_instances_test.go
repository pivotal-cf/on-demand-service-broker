// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_all_service_instances_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbroker"
)

func writeConfigFile(bUrl, bUsername, bPassword, sUrl, sUsername, sPassword string, pollingInterval int) string {
	const configContents = `---
broker_api:
  url: %s
  authentication:
    basic:
      username: %s
      password: %s
service_instances_api:
  url: %s
  authentication:
    basic:
      username: %s
      password: %s
polling_interval: %d
`

	file, err := ioutil.TempFile("", "config")
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	_, err = fmt.Fprintf(file, configContents, bUrl, bUsername, bPassword, sUrl, sUsername, sPassword, pollingInterval)
	Expect(err).NotTo(HaveOccurred())

	return file.Name()
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
		odb                           *mockhttp.Server
		configPath                    string
		testServiceInstancesAPIServer *httptest.Server
	)

	startUpgradeAllInstanceBinary := func() *gexec.Session {
		return helpers.StartBinaryWithParams(binaryPath, []string{"-configPath", configPath})
	}

	BeforeEach(func() {
		odb = mockbroker.New()
		odb.ExpectedBasicAuth(brokerUsername, brokerPassword)

		testServiceInstancesAPIServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})

	AfterEach(func() {
		odb.VerifyMocks()
		odb.Close()
		err := os.Remove(configPath)
		Expect(err).NotTo(HaveOccurred())
		testServiceInstancesAPIServer.Close()
	})

	Context("when service-instances-api is specified in the config", func() {
		It("exits successfully with one instance upgraded message", func() {
			operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{}}`
			instanceID := "service-instance-id"
			odb.VerifyAndMock(
				mockbroker.UpgradeInstance(instanceID).RespondsAcceptedWith(operationData),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationInProgress(),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationSucceeded(),
			)
			configPath = writeConfigFile(odb.URL, brokerUsername, brokerPassword,
				testServiceInstancesAPIServer.URL+serviceInstancesAPIURLPath,
				serviceInstancesAPIUsername, serviceInstancesAPIPassword, 1)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
			Expect(runningTool).To(gbytes.Say("Sleep interval until next attempt: 1s"))
			Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 1"))
		})

		It("returns unauthorißed when incorrect service instances API username provided", func() {
			configPath = writeConfigFile(odb.URL, brokerUsername, brokerPassword,
				testServiceInstancesAPIServer.URL+serviceInstancesAPIURLPath,
				"not-the-user", serviceInstancesAPIPassword, 1)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("error listing service instances: HTTP response status: 401 Unauthorized"))
		})
	})

	Context("when there is one service instance", func() {
		It("exits successfully with one instance upgraded message", func() {
			operationData := `{"BoshTaskID":1,"OperationType":"upgrade","PostDeployErrand":{}}`
			instanceID := "service-instance-id"
			odb.VerifyAndMock(
				mockbroker.ListInstances().RespondsOKWith(fmt.Sprintf(`[{"plan_id": "service-plan-id", "service_instance_id": "%s"}]`, instanceID)),
				mockbroker.UpgradeInstance(instanceID).RespondsAcceptedWith(operationData),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationInProgress(),
				mockbroker.LastOperation(instanceID, operationData).RespondWithOperationSucceeded(),
			)
			configPath = writeConfigFile(odb.URL, brokerUsername, brokerPassword, odb.URL+brokerServiceInstancesURLPath, brokerUsername, brokerPassword, 1)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool, 5*time.Second).Should(gexec.Exit(0))
			Expect(runningTool).To(gbytes.Say("Sleep interval until next attempt: 1s"))
			Expect(runningTool).To(gbytes.Say("Number of successful upgrades: 1"))
		})
	})

	Context("when the upgrade errors", func() {
		It("exits non-zero with the error message", func() {
			odb.VerifyAndMock(
				mockbroker.ListInstances().RespondsUnauthorizedWith(""),
			)

			configPath = writeConfigFile(odb.URL, brokerUsername, brokerPassword, odb.URL+brokerServiceInstancesURLPath, brokerUsername, brokerPassword, 1)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("error listing service instances: HTTP response status: 401 Unauthorized"))
		})
	})

	Context("when the upgrade tool is misconfigured", func() {
		It("fails with blank brokerUsername", func() {
			configPath = writeConfigFile(odb.URL, "", brokerPassword, odb.URL+brokerServiceInstancesURLPath, brokerUsername, brokerPassword, 1)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("the brokerUsername, brokerPassword and brokerUrl are required to function"))
		})

		It("fails with blank brokerPassword", func() {
			configPath = writeConfigFile(odb.URL, brokerUsername, "", odb.URL+brokerServiceInstancesURLPath, brokerUsername, brokerPassword, 1)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("the brokerUsername, brokerPassword and brokerUrl are required to function"))
		})

		It("fails with blank brokerUrl", func() {
			configPath = writeConfigFile("", brokerUsername, brokerPassword, odb.URL+brokerServiceInstancesURLPath, brokerUsername, brokerPassword, 1)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("the brokerUsername, brokerPassword and brokerUrl are required to function"))
		})

		It("fails with pollingInterval of zero", func() {
			configPath = writeConfigFile(odb.URL, brokerUsername, brokerPassword, odb.URL+brokerServiceInstancesURLPath, brokerUsername, brokerPassword, 0)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("the pollingInterval must be greater than zero"))
		})

		It("fails with pollingInterval less than zero", func() {
			configPath = writeConfigFile(odb.URL, brokerUsername, brokerPassword, odb.URL+brokerServiceInstancesURLPath, brokerUsername, brokerPassword, -123)
			runningTool := startUpgradeAllInstanceBinary()

			Eventually(runningTool).Should(gexec.Exit(1))
			Expect(runningTool).To(gbytes.Say("the pollingInterval must be greater than zero"))
		})
	})
})
