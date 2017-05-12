// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package orphan_deployments_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbroker"
)

const (
	brokerUsername = "broker-username"
	brokerPassword = "broker-password"
	errorMessage   = "error retrieving orphan deployments"
)

var _ = Describe("Orphan Deployments", func() {
	var odb *mockhttp.Server

	BeforeEach(func() {
		odb = mockbroker.New()
		odb.ExpectedBasicAuth(brokerUsername, brokerPassword)
	})

	AfterEach(func() {
		odb.VerifyMocks()
		odb.Close()
	})

	It("succeeds when no orphan deployments are detected", func() {
		odb.AppendMocks(mockbroker.OrphanDeployments().RespondsOKWith("[]"))

		session := startOrphanDeploymentsCmd(odb.URL)

		Eventually(session).Should(gexec.Exit(0))
		Expect(string(session.Out.Contents())).To(Equal("[]"))
	})

	It("fails with exit code 10 when orphan deployments are detected", func() {
		orphanBoshDeploymentsDetectedMessage := "Orphan BOSH deployments detected with no corresponding service instance in Cloud Foundry. Before deleting any deployment it is recommended to verify the service instance no longer exists in Cloud Foundry and any data is safe to delete."
		listOfDeployments := `[{"deployment_name":"service-instance_one"},{"deployment_name":"service-instance_two"}]`
		odb.AppendMocks(mockbroker.OrphanDeployments().RespondsOKWith(listOfDeployments))

		session := startOrphanDeploymentsCmd(odb.URL)

		Eventually(session).Should(gexec.Exit(10))
		Expect(string(session.Out.Contents())).To(Equal(listOfDeployments))
		Expect(session.Err).To(gbytes.Say(orphanBoshDeploymentsDetectedMessage))
	})

	It("fails when the broker credentials are unauthorised", func() {
		odb.AppendMocks(mockbroker.OrphanDeployments().RespondsUnauthorizedWith("unauthorized request"))

		session := startOrphanDeploymentsCmd(odb.URL)

		Eventually(session).Should(gexec.Exit(1))
		Expect(session.Err).To(SatisfyAll(
			gbytes.Say(errorMessage),
			gbytes.Say("%d", http.StatusUnauthorized),
		))
	})

	It("fails when the broker has an internal server error", func() {
		odb.AppendMocks(mockbroker.OrphanDeployments().RespondsInternalServerErrorWith("error message"))

		session := startOrphanDeploymentsCmd(odb.URL)

		Eventually(session).Should(gexec.Exit(1))
		Expect(session.Err).To(SatisfyAll(
			gbytes.Say(errorMessage),
			gbytes.Say("%d", http.StatusInternalServerError),
		))
	})

	It("fails when the broker is unavailable", func() {
		odb.Close()

		session := startOrphanDeploymentsCmd(odb.URL)

		Eventually(session).Should(gexec.Exit(1))
		Expect(session.Err).To(SatisfyAll(
			gbytes.Say(errorMessage),
			gbytes.Say("connection refused"),
		))
	})

	It("fails when the broker URL is invalid", func() {
		invalidURL := "$%#$%##$@#$#%$^&%^&$##$%@#"
		session := startOrphanDeploymentsCmd(invalidURL)

		Eventually(session).Should(gexec.Exit(1))
		Expect(string(session.Err.Contents())).To(SatisfyAll(
			ContainSubstring(errorMessage),
			ContainSubstring("invalid URL"),
		))
	})

	It("fails when the response is invalid JSON", func() {
		odb.AppendMocks(mockbroker.OrphanDeployments().RespondsOKWith("invalid json"))

		session := startOrphanDeploymentsCmd(odb.URL)

		Eventually(session).Should(gexec.Exit(1))
		Expect(session.Err).To(SatisfyAll(
			gbytes.Say(errorMessage),
			gbytes.Say("invalid character 'i'"),
		))
	})
})

func startOrphanDeploymentsCmd(url string) *gexec.Session {
	params := []string{
		"-brokerUsername", brokerUsername,
		"-brokerPassword", brokerPassword,
		"-brokerUrl", url,
	}
	cmd := exec.Command(binaryPath, params...)

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}
