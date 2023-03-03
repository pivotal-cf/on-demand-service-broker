// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package collect_service_metrics

import (
	"encoding/pem"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Collect Service Metrics", func() {
	const (
		brokerUsername = "broker username"
		brokerPassword = "broker password"
	)
	var (
		session *gexec.Session
		server  *ghttp.Server
		cmd     *exec.Cmd
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
		params := []string{
			"-brokerUsername", brokerUsername,
			"-brokerPassword", brokerPassword,
			"-brokerUrl", server.URL(),
		}
		cmd = exec.Command(binaryPath, params...)
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, time.Second*5).Should(gexec.Exit())
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when ODB responds with 200", func() {
		body := `[{"key":"/on-demand-broker/liteman/lite/total_instances","value":42,"unit":"count"}]`

		BeforeEach(func() {
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/mgmt/metrics"),
				ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
				ghttp.RespondWith(http.StatusOK, body, http.Header{}),
			))
		})

		It("exits with 0", func() {
			Expect(session.ExitCode()).To(Equal(0))
		})

		It("calls the broker /mgmt/metrics endpoint", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		It("returns the response body", func() {
			Expect(string(session.Out.Contents())).To(Equal(body))
		})
	})

	Context("when ODB responds with 200 in TLS", func() {
		body := `[{"key":"/on-demand-broker/liteman/lite/total_instances","value":42,"unit":"count"}]`
		var params []string

		BeforeEach(func() {
			server = ghttp.NewTLSServer()
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/mgmt/metrics"),
				ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
				ghttp.RespondWith(http.StatusOK, body, http.Header{}),
			))
		})

		When("skipTLSValidation is set to true", func() {
			BeforeEach(func() {
				params = []string{
					"-brokerUsername", brokerUsername,
					"-brokerPassword", brokerPassword,
					"-brokerUrl", server.URL(),
					"-disableTLSCertificateVerification=true",
				}
				cmd = exec.Command(binaryPath, params...)
			})

			It("collect the metrics successfully ", func() {
				Expect(session.ExitCode()).To(Equal(0))

				Expect(server.ReceivedRequests()).To(HaveLen(1))

				Expect(string(session.Out.Contents())).To(Equal(body))
			})
		})

		When("CA certificate is passed in", func() {
			BeforeEach(func() {
				rawPem := server.HTTPTestServer.Certificate().Raw
				pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rawPem})

				params = []string{
					"-brokerUsername", brokerUsername,
					"-brokerPassword", brokerPassword,
					"-brokerUrl", server.URL(),
					"-disableTLSCertificateVerification=false",
					"-brokerCACert", string(pemCert),
				}
				cmd = exec.Command(binaryPath, params...)
			})

			It("collect the metrics successfully ", func() {
				Expect(session.ExitCode()).To(Equal(0))

				Expect(server.ReceivedRequests()).To(HaveLen(1))

				Expect(string(session.Out.Contents())).To(Equal(body))
			})
		})

		When("skipTLSValidation is set to true and ca-cert passed as empty", func() {
			BeforeEach(func() {
				params = []string{
					"-brokerUsername", brokerUsername,
					"-brokerPassword", brokerPassword,
					"-brokerUrl", server.URL(),
					"-brokerCACert", `""`,
					"-disableTLSCertificateVerification=true",
				}
				cmd = exec.Command(binaryPath, params...)
			})

			It("collect the metrics successfully ", func() {
				Expect(session.ExitCode()).To(Equal(0))

				Expect(server.ReceivedRequests()).To(HaveLen(1))

				Expect(string(session.Out.Contents())).To(Equal(body))
			})
		})

	})

	Context("when the ODB responds with 500", func() {
		BeforeEach(func() {
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/mgmt/metrics"),
				ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
				ghttp.RespondWith(http.StatusInternalServerError, "", http.Header{}),
			))
		})

		It("exits with 0", func() {
			Expect(session.ExitCode()).To(Equal(0))
		})

		It("calls the broker /mgmt/metrics endpoint", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		It("returns the response body", func() {
			Expect(string(session.Out.Contents())).To(Equal("[]"))
		})
	})

	Context("when the ODB responds with 503", func() {
		BeforeEach(func() {
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/mgmt/metrics"),
				ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
				ghttp.RespondWith(http.StatusServiceUnavailable, "", http.Header{}),
			))
		})

		It("exits with 10", func() {
			Expect(session.ExitCode()).To(Equal(10))
		})

		It("calls the broker /mgmt/metrics endpoint", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when the ODB is unavailable", func() {
		BeforeEach(func() {
			server.Close()
		})

		It("exits with 1", func() {
			Expect(session.ExitCode()).To(Equal(1))
		})
	})

	Context("when the broker url is invalid", func() {
		BeforeEach(func() {
			params := []string{
				"-brokerUsername", brokerUsername,
				"-brokerPassword", brokerPassword,
				"-brokerUrl", "$%#$%##$@#$#%$^&%^&$##$%@#",
			}
			cmd = exec.Command(binaryPath, params...)
		})

		It("exits with 1", func() {
			Expect(session.ExitCode()).To(Equal(1))
		})
	})
})
