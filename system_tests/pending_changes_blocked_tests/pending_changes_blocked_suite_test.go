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
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var (
	serviceInstanceName string
	brokerInfo          bosh_helpers.BrokerInfo
)

var _ = BeforeSuite(func() {
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

var _ = AfterSuite(func() {
	cf.DeleteService(serviceInstanceName)
	bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
})

func TestPendingChangesBlockedTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PendingChangesBlocked Suite")
}

func disablePersistenceInFirstPlan(brokerManifest bosh.BoshManifest) bosh.BoshManifest {
	persistenceProperty := map[interface{}]interface{}{"persistence": false}
	brokerJob := bosh_helpers.FindJob(&brokerManifest, "broker", "broker")
	serviceCatalog := brokerJob.Properties["service_catalog"].(map[interface{}]interface{})
	dedicatedVMPlan := serviceCatalog["plans"].([]interface{})[0].(map[interface{}]interface{})
	dedicatedVMPlan["properties"] = persistenceProperty

	return brokerManifest
}
