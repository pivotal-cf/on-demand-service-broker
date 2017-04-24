// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package pending_changes_blocked_tests

import (
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("service instance with pending changes", func() {
	Context("when the app dev does NOT specify apply-changes", func() {
		var expectedErrMsg = "Service cannot be updated at this time, please try again later or contact your operator for more information"

		It("prevents a plan change", func() {
			session := cf.Cf("update-service", serviceInstanceName, "-p", "dedicated-high-memory-vm")
			Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())
			Expect(session).To(gbytes.Say(expectedErrMsg))
		})

		It("prevents setting arbitrary params", func() {
			session := cf.Cf("update-service", serviceInstanceName, "-c", `{"foo": "bar"}`)
			Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())
			Expect(session).To(gbytes.Say(expectedErrMsg))
		})
	})

	Context("when the app dev specifies apply-changes", func() {
		var expectedErrMsg = "'apply-changes' is not permitted. Contact your operator for more information"

		It("prevents a plan change", func() {
			session := cf.Cf("update-service", serviceInstanceName, "-p", "dedicated-high-memory-vm", "-c", `{"apply-changes": true}`)
			Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())
			Expect(session).To(gbytes.Say(expectedErrMsg))
		})

		It("prevents setting arbitrary params", func() {
			session := cf.Cf("update-service", serviceInstanceName, "-c", `{"apply-changes": true, "foo": "bar"}`)
			Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())
			Expect(session).To(gbytes.Say(expectedErrMsg))
		})

		It("prevents user from applying pending changes", func() {
			session := cf.Cf("update-service", serviceInstanceName, "-c", `{"apply-changes": true}`)
			Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())
			Expect(session).To(gbytes.Say(expectedErrMsg))
		})
	})
})
