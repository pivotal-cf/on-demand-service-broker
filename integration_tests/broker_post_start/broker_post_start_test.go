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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Broker Post-start Check", func() {
	const (
		brokerUsername = "broker username"
		brokerPassword = "broker password"
	)

	var (
		session *gexec.Session
		server  *ghttp.Server
		cmd     *exec.Cmd
		port    string
	)

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
			"-timeout", "1",
		}
		cmd = exec.Command(binaryPath, params...)
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
				ghttp.VerifyHeader(http.Header{"X-Broker-API-Version": []string{"2.13"}}),
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
			server.AppendHandlers(
				ghttp.RespondWith(http.StatusInternalServerError, nil),
				ghttp.RespondWith(http.StatusInternalServerError, nil),
				ghttp.RespondWith(http.StatusOK, nil),
			)

			params := []string{
				"-brokerUsername", brokerUsername,
				"-brokerPassword", brokerPassword,
				"-brokerPort", port,
				"-timeout", "3",
			}
			cmd = exec.Command(binaryPath, params...)
		})

		It("retries", func() {
			Expect(session.ExitCode()).To(Equal(0))
			Expect(server.ReceivedRequests()).To(HaveLen(3), "retries")
			Expect(session).To(gbytes.Say(fmt.Sprintf("expected status 200, was 500, from http://localhost:%s/v2/catalog", port)))
			Expect(session).To(gbytes.Say(fmt.Sprintf("expected status 200, was 500, from http://localhost:%s/v2/catalog", port)))
			Expect(session).To(gbytes.Say("Broker post-start check successful"))
		})
	})

	Context("when the ODB takes longer than the timeout to respond", func() {
		BeforeEach(func() {
			longRequestHandler := func(w http.ResponseWriter, req *http.Request) {
				time.Sleep(1100 * time.Millisecond)
			}

			server.AppendHandlers(longRequestHandler)
		})

		It("fails with error", func() {
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session).To(gbytes.Say("Broker post-start check failed"))
		})
	})

	Context("when the ODB does not respond", func() {
		BeforeEach(func() {
			server.Close()
		})

		It("fails with network error", func() {
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session).To(gbytes.Say("connection refused"))
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
			cmd = exec.Command(binaryPath, params...)
		})

		It("fails to start", func() {
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session).To(gbytes.Say("error creating request:"))
		})
	})
})
