// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package delete_all_service_instances_and_deregister_broker_tests

import (
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("purge instances and deregister broker", func() {
	serviceInstance1 := uuid.New()[:7]
	serviceInstance2 := uuid.New()[:7]
	serviceKeyName := uuid.New()[:7]
	testAppName := uuid.New()[:7]
	customTimeout := time.Second * 30

	BeforeEach(func() {
		Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, "dedicated-vm", serviceInstance1)).To(gexec.Exit(0))
		Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, "dedicated-high-memory-vm", serviceInstance2)).To(gexec.Exit(0))
		cf.AwaitServiceCreation(serviceInstance1)
		cf.AwaitServiceCreation(serviceInstance2)
	})

	AfterEach(func() {
		Expect(cf.CfWithTimeout(customTimeout, "unbind-service", testAppName, serviceInstance1)).To(gexec.Exit())
		Expect(cf.CfWithTimeout(customTimeout, "delete", testAppName, "-f", "-r")).To(gexec.Exit(0))
		Expect(cf.CfWithTimeout(customTimeout, "delete-service-key", serviceInstance1, serviceKeyName, "-f")).To(gexec.Exit())
		Expect(cf.CfWithTimeout(customTimeout, "delete-service", serviceInstance1, "-f")).To(gexec.Exit())
		Expect(cf.CfWithTimeout(customTimeout, "delete-service", serviceInstance2, "-f")).To(gexec.Exit())
		cf.AwaitServiceDeletion(serviceInstance1)
		cf.AwaitServiceDeletion(serviceInstance2)
	})

	It("deletes and unbinds all service instances", func() {
		Expect(cf.CfWithTimeout(time.Minute, "push", "-p", exampleAppPath, "--no-start", testAppName)).To(gexec.Exit(0))
		Expect(cf.CfWithTimeout(cf.CfTimeout, "bind-service", testAppName, serviceInstance1)).To(gexec.Exit(0))
		Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service-key", serviceInstance1, serviceKeyName)).To(gexec.Exit(0))

		session := bosh_helpers.RunErrand(
			brokerInfo.DeploymentName,
			"delete-all-service-instances-and-deregister-broker",
			gexec.Exit(0),
		)

		Expect(session.Buffer()).To(gbytes.Say("FINISHED PURGE INSTANCES AND DEREGISTER BROKER"))

		cf.AwaitServiceDeletion(serviceInstance1)
		cf.AwaitServiceDeletion(serviceInstance2)

		session = cf.CfWithTimeout(cf.CfTimeout, "marketplace", "-e", brokerInfo.ServiceName)
		Expect(session).To(gexec.Exit(0))
		Expect(session).Should(gbytes.Say("No service offerings found."))
	})
})
