// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package pending_changes_blocked_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("service instance with pending changes", func() {
	var expectedErrMsg = "The service broker has been updated, and this service instance is out of date. Please contact your operator."

	It("prevents a plan change", func() {
		session := cf.CfWithTimeout(cf.CfTimeout, "update-service", serviceInstanceName, "-p", "redis-plan-2")

		Expect(session).To(gexec.Exit())
		Expect(session).To(gbytes.Say(expectedErrMsg))
	})

	It("prevents setting arbitrary params", func() {
		session := cf.CfWithTimeout(cf.CfTimeout, "update-service", serviceInstanceName, "-c", `{"foo": "bar"}`)

		Expect(session).To(gexec.Exit())
		Expect(session).To(gbytes.Say(expectedErrMsg))
	})
})
