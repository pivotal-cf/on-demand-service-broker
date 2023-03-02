// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package orphan_deployments_test

import (
	"encoding/pem"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
)

const (
	brokerUsername = "broker-username"
	brokerPassword = "broker-password"
	errorMessage   = "error retrieving orphan deployments"
)

var _ = Describe("Orphan Deployments", func() {
	var (
		broker *ghttp.Server
		params []string
	)

	When("the broker is running HTTP", func() {
		BeforeEach(func() {
			broker = ghttp.NewServer()

			c := config.OrphanDeploymentsErrandConfig{
				BrokerAPI: config.BrokerAPI{
					URL: broker.URL(),
					Authentication: config.Authentication{
						Basic: config.UserCredentials{
							Username: brokerUsername,
							Password: brokerPassword,
						},
					},
				},
			}
			params = []string{
				"-configPath", write(c),
			}
		})

		AfterEach(func() {
			broker.Close()
		})

		It("calls the right broker endpoint with the configured credentials", func() {
			broker.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/mgmt/orphan_deployments"),
					ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
					ghttp.RespondWith(http.StatusOK, `[]`),
				),
			)
			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(0))
		})

		It("exits with 0 when no orphan deployments are detected", func() {
			broker.AppendHandlers(ghttp.RespondWith(http.StatusOK, `[]`))
			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal("[]\n"))
		})

		It("exits with code 10 when orphan deployments are detected", func() {
			orphanBoshDeploymentsDetectedMessage := "Orphan BOSH deployments detected with no corresponding service instance in the platform. " +
				"Before deleting any deployment it is recommended to verify the service instance no longer exists in the platform and any data is safe to delete."
			listOfDeployments := `[{"deployment_name":"service-instance_one"},{"deployment_name":"service-instance_two"}]`

			broker.AppendHandlers(ghttp.RespondWith(http.StatusOK, listOfDeployments))
			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(10))
			Expect(string(session.Out.Contents())).To(ContainSubstring(listOfDeployments))
			Expect(session.Err).To(gbytes.Say(orphanBoshDeploymentsDetectedMessage))
		})

		It("fails when the broker credentials are unauthorised", func() {
			broker.AppendHandlers(ghttp.RespondWith(http.StatusUnauthorized, ""))

			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(SatisfyAll(
				gbytes.Say(errorMessage),
				gbytes.Say("%d", http.StatusUnauthorized),
			))
		})

		It("fails when the broker has an internal server error", func() {
			broker.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, "error message"))

			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(SatisfyAll(
				gbytes.Say(errorMessage),
				gbytes.Say("%d", http.StatusInternalServerError),
			))
		})

		It("fails when the broker is unavailable", func() {
			broker.Close()

			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(SatisfyAll(
				gbytes.Say(errorMessage),
				gbytes.Say("connection refused"),
			))
		})

		It("fails when the response is invalid JSON", func() {
			broker.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.RespondWith(http.StatusOK, "invalid json"),
				),
			)

			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(SatisfyAll(
				gbytes.Say(errorMessage),
				gbytes.Say("invalid character 'i'"),
			))
		})

	})

	When("the broker is running HTTPS", func() {
		var c config.OrphanDeploymentsErrandConfig

		BeforeEach(func() {
			broker = ghttp.NewTLSServer()

			c = config.OrphanDeploymentsErrandConfig{
				BrokerAPI: config.BrokerAPI{
					URL: broker.URL(),
					Authentication: config.Authentication{
						Basic: config.UserCredentials{
							Username: brokerUsername,
							Password: brokerPassword,
						},
					},
				},
			}

			broker.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/mgmt/orphan_deployments"),
					ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
					ghttp.RespondWith(http.StatusOK, `[]`),
				),
			)
		})

		AfterEach(func() {
			broker.Close()
		})

		It("can communicate with the broker", func() {
			rawPem := broker.HTTPTestServer.Certificate().Raw
			pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rawPem})
			c.BrokerAPI.TLS = config.ErrandTLSConfig{CACert: string(pemCert)}

			params = []string{"-configPath", write(c)}

			session := helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(session).Should(gexec.Exit(0))
		})

		It("skips ssl cert verification", func() {
			c.BrokerAPI.TLS = config.ErrandTLSConfig{DisableSSLCertVerification: true}

			params = []string{"-configPath", write(c)}

			session := helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	When("invoking the binary with broken arguments", func() {
		It("fails when the config path is not provided", func() {
			params = []string{}
			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("-configPath must be given as argument"))
		})

		It("fails when the config path is provided, but empty", func() {
			params = []string{"-configPath"}
			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(2))
			Expect(session.Err).To(gbytes.Say("flag needs an argument: -configPath"))
		})

		It("fails when the config path can't be read", func() {
			params = []string{"-configPath", "/not/a/file"}
			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("open /not/a/file: no such file or directory"))
		})

		It("fails when the config file can't be correctly parsed", func() {
			params = []string{"-configPath", write([]byte("--1--"))}
			session := helpers.StartBinaryWithParams(binaryPath, params)

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err).To(gbytes.Say("failed to unmarshal errand config"))
		})
	})

})
