// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("broker post start checks", func() {
	var session *gexec.Session
	var server *ghttp.Server
	var cmd *exec.Cmd
	var port string

	BeforeEach(func() {
		server = ghttp.NewServer()
		serverURL, err := url.Parse(server.URL())
		Expect(err).NotTo(HaveOccurred())
		parts := strings.Split(serverURL.Host, ":")
		Expect(parts).To(HaveLen(2))
		port = parts[1]

		params := []string{
			"-brokerUsername", brokerUsername,
			"-brokerPassword", brokerPassword,
			"-brokerPort", port,
			"-timeout", "5",
		}
		cmd = exec.Command(brokerPostStartPath, params...)
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, time.Second*6).Should(gexec.Exit())
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when ODB responds with 200", func() {
		BeforeEach(func() {
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/catalog"),
				ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
				ghttp.RespondWith(http.StatusOK, nil),
			))
		})

		It("exits successfully", func() {
			Expect(session.ExitCode()).To(Equal(0))
			Expect(server.ReceivedRequests()).To(HaveLen(1))
			Expect(session).To(gbytes.Say("Starting broker post-start check, waiting for broker to start serving catalog."))
			Expect(session).To(gbytes.Say("Broker post-start check successful"))
		})
	})

	Context("when the ODB responds with 500", func() {
		BeforeEach(func() {
			for i := 0; i < 5; i++ {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, nil))
			}
		})

		It("exits with an error", func() {
			By("exiting with 1")
			Expect(session.ExitCode()).To(Equal(1))

			By("calling the broker /v2/catalog endpoint at least four times")
			Expect(len(server.ReceivedRequests())).To(BeNumerically(">=", 4))

			By("outputting each failed attempt")
			Expect(session).To(gbytes.Say(fmt.Sprintf("expected status 200, was 500, from http://localhost:%s/v2/catalog", port)))
			Expect(session).To(gbytes.Say(fmt.Sprintf("expected status 200, was 500, from http://localhost:%s/v2/catalog", port)))
			Expect(session).To(gbytes.Say(fmt.Sprintf("expected status 200, was 500, from http://localhost:%s/v2/catalog", port)))
			Expect(session).To(gbytes.Say(fmt.Sprintf("expected status 200, was 500, from http://localhost:%s/v2/catalog", port)))

			By("outputing and error message")
			Expect(session.Out.Contents()).To(ContainSubstring("Broker post-start check failed"))
		})
	})

	Context("when the ODB takes longer than the timeout to respond", func() {
		BeforeEach(func() {
			longRequestHandler := func(w http.ResponseWriter, req *http.Request) {
				time.Sleep(6 * time.Second)
			}

			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/catalog"),
				ghttp.VerifyBasicAuth(brokerUsername, brokerPassword),
				longRequestHandler,
			))
		})

		It("exits with error", func() {
			By("exiting with 1")
			Expect(session.ExitCode()).To(Equal(1))

			By("outputing and error message")
			Expect(session).To(gbytes.Say("Broker post-start check failed"))
		})
	})

	Context("when the ODB does not respond", func() {
		BeforeEach(func() {
			server.Close()
		})

		It("exits with error", func() {
			By("exiting with 1")
			Expect(session.ExitCode()).To(Equal(1))

			By("outputing the network error")
			Expect(session).To(gbytes.Say("connection refused"))

			By("outputing and error message")
			Expect(session).To(gbytes.Say("Broker post-start check failed"))
		})
	})

	Context("when the broker port is invalid", func() {
		BeforeEach(func() {
			params := []string{
				"-brokerUsername", brokerUsername,
				"-brokerPassword", brokerPassword,
				"-brokerPort", "$%#$%##$@#$#%$^&%^&$##$%@#",
			}
			cmd = exec.Command(brokerPostStartPath, params...)
		})

		It("outputs an error message", func() {
			Expect(session).To(gbytes.Say("error creating request:"))
		})

		It("exits with 1", func() {
			Expect(session.ExitCode()).To(Equal(1))
		})
	})
})
