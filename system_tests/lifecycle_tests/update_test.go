// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package lifecycle_tests

import (
	"github.com/onsi/gomega/gexec"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Updating a service instance", func() {
	var (
		serviceName string
	)

	BeforeEach(func() {
		serviceName = newServiceName()
		cf.CreateService(serviceOffering, "dedicated-vm", serviceName, "")
	})

	AfterEach(func() {
		cf.DeleteService(serviceName)
	})

	It("updates successfully", func() {
		Eventually(
			cf.Cf("update-service", serviceName, "-p", "dedicated-high-memory-vm"),
			cf.CfTimeout,
		).Should(gexec.Exit(0))
		cf.AwaitServiceUpdate(serviceName)
	})
})
