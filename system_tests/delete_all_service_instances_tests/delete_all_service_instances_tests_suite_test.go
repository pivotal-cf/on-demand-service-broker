// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package delete_all_service_instances_tests

import (
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/credhub_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/env_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
)

func TestDeleteAllInstancesTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DeleteAllInstancesTests Suite")
}

var (
	brokerInfo BrokerInfo
	credhubCLI *credhub_helpers.CredHubCLI
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]

	brokerInfo = DeployAndRegisterBroker(
		"-delete-all-service-instances-"+uniqueID,
		BrokerDeploymentOptions{},
		service_helpers.Redis,
		[]string{"basic_service_catalog.yml", "enable_secure_manifests.yml"},
	)

	setupCredhubCLIClient()
})

var _ = AfterSuite(func() {
	DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
})

func setupCredhubCLIClient() {
	err := env_helpers.ValidateEnvVars("CREDHUB_CLIENT", "CREDHUB_SECRET")
	Expect(err).NotTo(HaveOccurred())
	credhubCLI = credhub_helpers.NewCredHubCLI(os.Getenv("CREDHUB_CLIENT"), os.Getenv("CREDHUB_SECRET"))
}
