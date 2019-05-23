// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deregister_broker_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-service-broker/registrar"
	"gopkg.in/yaml.v2"
)

var _ = Describe("DeregisterBroker", func() {
	const (
		cfAccessToken     = "cf-oauth-token"
		cfUaaClientID     = "cf-uaa-client-id"
		cfUaaClientSecret = "cf-uaa-client-secret"

		serviceBrokerGUID = "some-service-broker-guid"
		serviceBrokerName = "some-broker-name"

		timeout = time.Second * 5
	)

	var (
		cfAPI              *mockhttp.Server
		cfUAA              *mockuaa.ClientCredentialsServer
		configuration      registrar.Config
		configFilePath     string
		deregistrarSession *gexec.Session
	)

	BeforeEach(func() {
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, cfAccessToken)

		configuration = registrar.Config{
			DisableSSLCertVerification: true,
			CF: config.CF{
				URL: cfAPI.URL,
				Authentication: config.Authentication{
					UAA: config.UAAAuthentication{
						URL: cfUAA.URL,
						ClientCredentials: config.ClientCredentials{
							ID:     cfUaaClientID,
							Secret: cfUaaClientSecret,
						},
					},
				},
			},
		}

		configYAML, err := yaml.Marshal(configuration)
		Expect(err).ToNot(HaveOccurred())

		configFilePath = helpers.WriteConfig(configYAML, tempDir)
	})

	AfterEach(func() {
		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
	})

	It("fails when broker name is not provided", func() {
		params := []string{"-configFilePath", configFilePath}
		deregistrarSession = helpers.StartBinaryWithParams(binaryPath, params)

		Eventually(deregistrarSession, timeout).Should(gexec.Exit(1), "broker name not provided")
		Eventually(deregistrarSession).Should(gbytes.Say("Missing argument -brokerName"))
	})

	It("fails when config file path is not provided", func() {
		params := []string{"-brokerName", serviceBrokerName}
		deregistrarSession = helpers.StartBinaryWithParams(binaryPath, params)

		Eventually(deregistrarSession, timeout).Should(gexec.Exit(1), "config file path not provided")
		Eventually(deregistrarSession).Should(gbytes.Say("Missing argument -configFilePath"))
	})

	It("fails when configFilePath cannot be read", func() {
		params := []string{"-configFilePath", "/tmp/foo/bar", "-brokerName", serviceBrokerName}
		deregistrarSession = helpers.StartBinaryWithParams(binaryPath, params)

		Eventually(deregistrarSession, timeout).Should(gexec.Exit(1), "configFilePath cannot be read")
		Eventually(deregistrarSession).Should(gbytes.Say("Error reading config file:"))
	})

	It("fails when the config is not valid yaml", func() {
		configFilePath := helpers.WriteConfig([]byte("not valid yaml"), tempDir)
		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		deregistrarSession = helpers.StartBinaryWithParams(binaryPath, params)

		Eventually(deregistrarSession, timeout).Should(gexec.Exit(1), "config is not valid yaml")
		Eventually(deregistrarSession).Should(gbytes.Say("Invalid config file:"))
	})

	It("deregisters the broker", func() {
		cfAPI.VerifyAndMock(
			mockcfapi.ListServiceBrokers().RespondsWithBrokers(serviceBrokerName, serviceBrokerGUID),
			mockcfapi.DeregisterBroker(serviceBrokerGUID).RespondsNoContent(),
		)

		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		deregistrarSession := helpers.StartBinaryWithParams(binaryPath, params)
		Eventually(deregistrarSession, timeout).Should(gexec.Exit(0))
		Expect(deregistrarSession).To(gbytes.Say("FINISHED DEREGISTER BROKER"))
	})

	It("fails when the deregistrar fails", func() {
		cfAPI.VerifyAndMock(
			mockcfapi.ListServiceBrokers().RespondsWithBrokers(serviceBrokerName, serviceBrokerGUID),
			mockcfapi.DeregisterBroker(serviceBrokerGUID).RespondsInternalServerErrorWith("failed"),
		)

		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		deregistrarSession = helpers.StartBinaryWithParams(binaryPath, params)
		Eventually(deregistrarSession, timeout).Should(gexec.Exit(1))
		Eventually(deregistrarSession).Should(gbytes.Say("Failed to deregister broker"))
	})

	It("succeeds when the broker doesn't exist", func() {
		cfAPI.VerifyAndMock(
			mockcfapi.ListServiceBrokers().RespondsWithBrokers("not-this-broker", "other-id"),
		)

		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		deregistrarSession = helpers.StartBinaryWithParams(binaryPath, params)
		Eventually(deregistrarSession, timeout).Should(gexec.Exit(0))
		Eventually(deregistrarSession).Should(gbytes.Say("No service broker found with name: " + serviceBrokerName))
	})
})
