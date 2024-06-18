// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package delete_all_service_instances_and_deregister_broker_tests

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var (
	exampleAppPath string
	brokerInfo     bosh_helpers.BrokerInfo
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]
	brokerInfo = bosh_helpers.DeployAndRegisterBroker(
		"-delete-all-"+uniqueID,
		bosh_helpers.BrokerDeploymentOptions{},
		service_helpers.Redis,
		[]string{"update_service_catalog.yml"})

	exampleAppPath = envMustHave("REDIS_EXAMPLE_APP_PATH")
})

var _ = AfterSuite(func() {
	if os.Getenv("KEEP_ALIVE") != "true" {
		bosh_helpers.DeregisterAndDeleteBrokerSilently(brokerInfo.DeploymentName)
	}
})

func TestDeleteAllInstancesTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DeleteAllInstancesTests Suite")
}

func envMustHave(envVar string) string {
	envVal := os.Getenv(envVar)
	Expect(envVal).ToNot(BeEmpty(), fmt.Sprintf("must set %s", envVar))
	return envVal
}
