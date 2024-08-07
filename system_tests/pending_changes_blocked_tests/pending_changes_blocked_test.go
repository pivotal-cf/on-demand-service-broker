// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package pending_changes_blocked_tests

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("service instance with pending changes", Ordered, func() {
	const expectedErrMsg = "The service broker has been updated, and this service instance is out of date. Please contact your operator."

	getServiceDeploymentName := func(serviceInstanceName string) string {
		getInstanceDetailsCmd := cf.CfWithTimeout(cf.CfTimeout, "service", serviceInstanceName, "--guid")
		Expect(getInstanceDetailsCmd).To(gexec.Exit(0))
		re := regexp.MustCompile("(?m)^[[:alnum:]]{8}-[[:alnum:]-]*$")
		serviceGUID := re.FindString(string(getInstanceDetailsCmd.Out.Contents()))
		serviceInstanceID := strings.TrimSpace(serviceGUID)
		return fmt.Sprintf("service-instance_%s", serviceInstanceID)
	}

	runBrokerCmd := func(command string, w io.Writer) {
		cmd := exec.Command("bosh", "-d", brokerInfo.DeploymentName, "ssh", "--results", "--column=Stdout", "broker", "-c", command)
		session, err := gexec.Start(cmd, io.MultiWriter(GinkgoWriter, w), GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "failed to run ssh")
		Eventually(session, "5m").Should(gexec.Exit(), "Expected to SSH successfully")
	}

	It("prevents a plan change", func() {
		session := cf.CfWithTimeout(cf.CfTimeout, "update-service", serviceInstanceName, "-p", "redis-plan-2")

		Expect(session).To(gexec.Exit())
		Expect(session).To(gbytes.Say(expectedErrMsg), `Expected the update-service to be rejected when the instance was out-of-date`)

		serviceInstanceDeploymentName := getServiceDeploymentName(serviceInstanceName)
		manifestDirectory := "/var/vcap/data/broker/manifest/" + serviceInstanceDeploymentName
		var output bytes.Buffer
		runBrokerCmd(
			"sudo zdiff -u "+manifestDirectory+"/old_manifest.yml.gz "+manifestDirectory+"/new_manifest.yml.gz",
			&output,
		)

		// very roughly match the expected diff to see we recorded the expected information
		// based on the changes from disablePersistenceInFirstPlan(brokerManifest)
		Expect(output.String()).To(SatisfyAll(
			ContainSubstring(`-        persistence: "yes"`),
			ContainSubstring(`+        persistence: "no"`),
			ContainSubstring(`-  canaries: 4`),
			ContainSubstring(`+  canaries: 1`),
			ContainSubstring(`-  max_in_flight: 4`),
			ContainSubstring(`+  max_in_flight: 1`),
		))
	})

	It("prevents setting arbitrary params", func() {
		session := cf.CfWithTimeout(cf.CfTimeout, "update-service", serviceInstanceName, "-c", `{"foo": "bar"}`)

		Expect(session).To(gexec.Exit())
		Expect(session).To(gbytes.Say(expectedErrMsg))

		serviceInstanceDeploymentName := getServiceDeploymentName(serviceInstanceName)
		manifestDirectory := "/var/vcap/data/broker/manifest/" + serviceInstanceDeploymentName
		var output bytes.Buffer
		runBrokerCmd(
			"sudo zdiff -u "+manifestDirectory+"/old_manifest.yml.gz "+manifestDirectory+"/new_manifest.yml.gz",
			&output,
		)

		// very roughly match the expected diff to see we recorded the expected information
		// based on the changes from disablePersistenceInFirstPlan(brokerManifest)
		Expect(output.String()).To(SatisfyAll(
			ContainSubstring(`-        persistence: "yes"`),
			ContainSubstring(`+        persistence: "no"`),
			ContainSubstring(`-  canaries: 4`),
			ContainSubstring(`+  canaries: 1`),
			ContainSubstring(`-  max_in_flight: 4`),
			ContainSubstring(`+  max_in_flight: 1`),
		))
	})

	It("cleans up the saved manifests directory when an instance is deleted", func() {
		cf.DeleteService(serviceInstanceName)

		var output bytes.Buffer
		runBrokerCmd("sudo ls /var/vcap/data/broker/manifest", &output)

		// bosh ssh always writes "-" as a stub for empty output
		Expect(strings.TrimSpace(output.String())).To(Equal("-"))
	})
})
