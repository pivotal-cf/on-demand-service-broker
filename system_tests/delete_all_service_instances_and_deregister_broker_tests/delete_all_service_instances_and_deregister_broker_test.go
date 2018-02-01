// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package delete_all_service_instances_and_deregister_broker_tests

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("purge instances and deregister broker", func() {
	serviceInstance1 := uuid.New()[:7]
	serviceInstance2 := uuid.New()[:7]
	serviceKeyName := uuid.New()[:7]
	testAppName := uuid.New()[:7]

	BeforeEach(func() {
		Eventually(cf.Cf("create-service", serviceOffering, "dedicated-vm", serviceInstance1), cf.CfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("create-service", serviceOffering, "dedicated-high-memory-vm", serviceInstance2), cf.CfTimeout).Should(gexec.Exit(0))
		cf.AwaitServiceCreation(serviceInstance1)
		cf.AwaitServiceCreation(serviceInstance2)
	})

	AfterEach(func() {
		Eventually(cf.Cf("unbind-service", testAppName, serviceInstance1), time.Second*30).Should(gexec.Exit())
		Eventually(cf.Cf("delete", testAppName, "-f", "-r"), time.Second*30).Should(gexec.Exit(0))
		Eventually(cf.Cf("delete-service-key", serviceInstance1, serviceKeyName, "-f"), time.Second*30).Should(gexec.Exit())
		Eventually(cf.Cf("delete-service", serviceInstance1, "-f"), time.Second*30).Should(gexec.Exit())
		Eventually(cf.Cf("delete-service", serviceInstance2, "-f"), time.Second*30).Should(gexec.Exit())
		cf.AwaitServiceDeletion(serviceInstance1)
		cf.AwaitServiceDeletion(serviceInstance2)
	})

	It("deletes and unbinds all service instances", func() {
		Eventually(cf.Cf("push", "-p", exampleAppPath, "--no-start", testAppName), time.Minute).Should(gexec.Exit(0))
		Eventually(cf.Cf("bind-service", testAppName, serviceInstance1), cf.CfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("create-service-key", serviceInstance1, serviceKeyName), cf.CfTimeout).Should(gexec.Exit(0))

		output := boshClient.RunErrand(brokerBoshDeploymentName, "delete-all-service-instances-and-deregister-broker", []string{}, "")

		Expect(output.StdOut).To(ContainSubstring("FINISHED PURGE INSTANCES AND DEREGISTER BROKER"))
		Expect(output.ExitCode).To(Equal(0))

		cf.AwaitServiceDeletion(serviceInstance1)
		cf.AwaitServiceDeletion(serviceInstance2)
		session := cf.Cf("marketplace", "-s", serviceOffering)
		Eventually(session, cf.CfTimeout).Should(gexec.Exit(1))
		Expect(session).Should(gbytes.Say("Service offering %s not found", serviceOffering))
	})
})
