// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_post_start

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Broker Post-start Check", func() {
	const (
		brokerUsername           = "broker username"
		brokerPassword           = "broker password"
		pathToValidConfig        = "test_assets/good_config.yml"
		pathToValidConfigWithTLS = "test_assets/good_config_with_tls.yml"
	)

	var (
		session *gexec.Session
		server  *ghttp.Server
		port    string
	)

	Context("when the broker is configured without TLS", func() {
		BeforeEach(func() {
			server = ghttp.NewServer()
			port = extractPort(server.URL())
			appendDefaultHandlers(server, brokerUsername, brokerPassword)
		})

		AfterEach(func() {
			server.Close()
		})

		It("exits successfully when the ODB responds with 200", func() {
			session = executeBinary(brokerUsername, brokerPassword, port, pathToValidConfig, "1")

			Expect(session.ExitCode()).To(Equal(0))
			Expect(server.ReceivedRequests()).To(HaveLen(1))
			Expect(session).To(gbytes.Say("Starting broker post-start check, waiting for broker to start serving catalog."))
			Expect(session).To(gbytes.Say("Broker post-start check successful"))
		})
	})

	Context("when the broker is configured with TLS", func() {
		BeforeEach(func() {
			server = ghttp.NewTLSServer()
			port = extractPort(server.URL())
			appendDefaultHandlers(server, brokerUsername, brokerPassword)
			session = executeBinary(brokerUsername, brokerPassword, port, pathToValidConfigWithTLS, "1")
		})

		AfterEach(func() {
			server.Close()
		})

		It("exits successfully", func() {
			Expect(session.ExitCode()).To(Equal(0))
			Expect(server.ReceivedRequests()).To(HaveLen(1))
			Expect(session).To(gbytes.Say("Starting broker post-start check, waiting for broker to start serving catalog."))
			Expect(session).To(gbytes.Say("Broker post-start check successful"))
		})
	})

	Describe("error handling", func() {
		BeforeEach(func() {
			server = ghttp.NewServer()
			port = extractPort(server.URL())
		})

		AfterEach(func() {
			server.Close()
		})

		It("retries when the ODB responds with 500", func() {
			server.AppendHandlers(
				ghttp.RespondWith(http.StatusInternalServerError, nil),
				ghttp.RespondWith(http.StatusInternalServerError, nil),
				ghttp.RespondWith(http.StatusOK, nil),
			)

			session = executeBinary(brokerUsername, brokerPassword, port, pathToValidConfig, "3")

			Expect(session.ExitCode()).To(Equal(0), "wrong exit code")
			Expect(server.ReceivedRequests()).To(HaveLen(3), "retries")
			Expect(session).To(gbytes.Say(fmt.Sprintf("expected status 200, was 500, from http://localhost:%s/v2/catalog", port)))
			Expect(session).To(gbytes.Say(fmt.Sprintf("expected status 200, was 500, from http://localhost:%s/v2/catalog", port)))
			Expect(session).To(gbytes.Say("Broker post-start check successful"))
		})

		It("fails with error when the ODB takes longer than the timeout to respond", func() {
			longRequestHandler := func(w http.ResponseWriter, req *http.Request) {
				time.Sleep(1100 * time.Millisecond)
			}
			server.AppendHandlers(longRequestHandler)

			session = executeBinary(brokerUsername, brokerPassword, port, pathToValidConfig, "1")

			Expect(session.ExitCode()).To(Equal(1))
			Expect(session).To(gbytes.Say("Broker post-start check failed"))
		})

		It("fails with network error when the ODB does not respond", func() {
			server.Close()

			session = executeBinary(brokerUsername, brokerPassword, port, pathToValidConfig, "1")

			Expect(session.ExitCode()).To(Equal(1))
			Expect(session).To(gbytes.Say("connection refused"))
			Expect(session).To(gbytes.Say("Broker post-start check failed"))
		})

		It("fails to start when the broker port is invalid", func() {
			session = executeBinary(brokerUsername, brokerPassword, "$%#$%##$@#$#%$^&%^&$##$%@#", pathToValidConfig, "1")

			Expect(session.ExitCode()).To(Equal(1))
			Expect(session).To(gbytes.Say("error creating request:"))
		})

		It("fails to start when the broker config file path is invalid", func() {
			session = executeBinary(brokerUsername, brokerPassword, port, "not a real path", "1")

			Expect(session.ExitCode()).To(Equal(1))
			Expect(session).To(gbytes.Say("no such file or directory"))
		})
	})
})

func extractPort(stringURL string) string {
	serverURL, err := url.Parse(stringURL)
	Expect(err).NotTo(HaveOccurred())
	parts := strings.Split(serverURL.Host, ":")
	Expect(parts).To(HaveLen(2))
	return parts[1]
}

func executeBinary(brokerUsername, brokerPassword, port, pathToConfig, timeout string) *gexec.Session {
	params := []string{
		"-brokerUsername", brokerUsername,
		"-brokerPassword", brokerPassword,
		"-brokerPort", port,
		"-timeout", timeout,
		"-configFilePath", pathToConfig,
	}
	cmd := exec.Command(binaryPath, params...)

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(session, time.Second*5).Should(gexec.Exit())
	return session
}

func appendDefaultHandlers(server *ghttp.Server, brokerUsername, brokerPassword string) {
	server.AppendHandlers(ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", "/v2/catalog"),
		ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
		ghttp.VerifyHeader(http.Header{"X-Broker-API-Version": []string{"2.13"}}),
		ghttp.RespondWith(http.StatusOK, nil),
	))
}
