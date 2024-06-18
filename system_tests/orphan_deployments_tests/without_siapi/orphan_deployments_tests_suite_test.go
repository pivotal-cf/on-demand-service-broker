// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package orphan_deployments_tests

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var brokerInfo bosh.BrokerInfo

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]

	deploymentOptions := bosh.BrokerDeploymentOptions{
		BrokerTLS: true,
	}

	brokerInfo = bosh.DeployAndRegisterBroker(
		"-orphan-deployment-without-siapi-"+uniqueID,
		deploymentOptions,
		service_helpers.Redis,
		[]string{"update_service_catalog.yml", "update_orphan_deployments_job.yml"})
})

var _ = AfterSuite(func() {
	bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
})

func TestOrphanDeploymentsTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OrphanDeploymentsTests Suite")
}
