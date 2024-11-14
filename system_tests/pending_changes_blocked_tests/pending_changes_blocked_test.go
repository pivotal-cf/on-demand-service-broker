// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package pending_changes_blocked_tests

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

func TestPendingChangesBlockedTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PendingChangesBlocked Suite")
}

var _ = Describe("service instance with pending changes", Ordered, func() {
	const expectedErrMsg = "The service broker has been updated, and this service instance is out of date. Please contact your operator."

	var (
		serviceInstanceName string
		brokerInfo          bosh_helpers.BrokerInfo
	)

	When("pending changes are blocked", func() {
		BeforeAll(func() {
			uniqueID := uuid.New()[:6]
			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				"-pending-changes-"+uniqueID,
				bosh_helpers.BrokerDeploymentOptions{},
				service_helpers.Redis,
				[]string{"update_service_catalog.yml"})

			serviceInstanceName = "service" + brokerInfo.TestSuffix
			cf.CreateService(brokerInfo.ServiceName, "redis-small", serviceInstanceName, "")

			By("causing pending changes for the service instance")
			deployedManifest := bosh_helpers.GetManifest(brokerInfo.DeploymentName)
			updatedManifest := disablePersistenceInFirstPlan(deployedManifest)

			By("deploying the modified broker manifest")
			bosh_helpers.RedeployBroker(brokerInfo.DeploymentName, brokerInfo.URI, updatedManifest)
		})

		AfterAll(func() {
			cf.DeleteService(serviceInstanceName)
			bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
		})

		It("prevents a plan change", func() {
			session := cf.CfWithTimeout(cf.CfTimeout, "update-service", serviceInstanceName, "-p", "redis-plan-2")

			Expect(session).To(gexec.Exit())
			Expect(session).To(gbytes.Say(expectedErrMsg), `Expected the update-service to be rejected when the instance was out-of-date`)
		})

		It("prevents setting arbitrary params", func() {
			session := cf.CfWithTimeout(cf.CfTimeout, "update-service", serviceInstanceName, "-c", `{"foo": "bar"}`)

			Expect(session).To(gexec.Exit())
			Expect(session).To(gbytes.Say(expectedErrMsg))
		})
	})

	When("pending changes are allowed", func() {
		BeforeAll(func() {
			uniqueID := uuid.New()[:6]
			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				"-pending-changes-"+uniqueID,
				bosh_helpers.BrokerDeploymentOptions{},
				service_helpers.Redis,
				[]string{"update_service_catalog.yml", "skip_check_for_pending_changes.yml"})

			serviceInstanceName = "service" + brokerInfo.TestSuffix
			cf.CreateService(brokerInfo.ServiceName, "redis-small", serviceInstanceName, "")

			By("causing pending changes for the service instance")
			deployedManifest := bosh_helpers.GetManifest(brokerInfo.DeploymentName)
			updatedManifest := disablePersistenceInFirstPlan(deployedManifest)

			By("deploying the modified broker manifest")
			bosh_helpers.RedeployBroker(brokerInfo.DeploymentName, brokerInfo.URI, updatedManifest)
		})

		AfterAll(func() {
			cf.DeleteService(serviceInstanceName)
			bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
		})

		It("succeeds", func() {
			session := cf.CfWithTimeout(cf.LongCfTimeout, "update-service", serviceInstanceName, "-p", "redis-plan-2")
			Expect(session).To(gexec.Exit(0))

			cf.AwaitServiceUpdate(serviceInstanceName)

			getInstanceDetailsCmd := cf.CfWithTimeout(cf.CfTimeout, "service", serviceInstanceName)
			Expect(getInstanceDetailsCmd).To(gexec.Exit(0))
			Expect(getInstanceDetailsCmd).To(gbytes.Say(`redis-plan-2`))
		})
	})
})

func disablePersistenceInFirstPlan(brokerManifest bosh.BoshManifest) bosh.BoshManifest {
	persistenceProperty := map[interface{}]interface{}{"persistence": false}
	brokerJob := bosh_helpers.FindJob(&brokerManifest, "broker", "broker")
	serviceCatalog := brokerJob.Properties["service_catalog"].(map[interface{}]interface{})
	dedicatedVMPlan := serviceCatalog["plans"].([]interface{})[0].(map[interface{}]interface{})
	dedicatedVMPlan["properties"] = persistenceProperty

	return brokerManifest
}
