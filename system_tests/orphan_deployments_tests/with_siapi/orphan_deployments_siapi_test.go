// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orphan_deployments_tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/brokerapi_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/siapi_helpers"
)

var _ = Describe("orphan deployments errand", func() {
	Context("when there are two deployments and one is an orphan", func() {
		var (
			orphanInstanceGUID    string
			nonOrphanInstanceGUID string
			brokerAPIClient       *brokerapi_helpers.BrokerAPIClient
		)

		BeforeEach(func() {
			brokerAPIClient = &brokerapi_helpers.BrokerAPIClient{
				URI:      brokerInfo.URI,
				Username: brokerInfo.BrokerUsername,
				Password: brokerInfo.BrokerPassword,
			}
			orphanInstanceGUID = "instance-to-purge-" + uuid.New()
			nonOrphanInstanceGUID = "instance-to-keep-" + uuid.New()

			provResp1 := brokerAPIClient.Provision(orphanInstanceGUID, brokerInfo.ServiceID, brokerInfo.PlanID)
			provResp2 := brokerAPIClient.Provision(nonOrphanInstanceGUID, brokerInfo.ServiceID, brokerInfo.PlanID)

			brokerAPIClient.PollLastOperation(orphanInstanceGUID, provResp1.OperationData)
			brokerAPIClient.PollLastOperation(nonOrphanInstanceGUID, provResp2.OperationData)
		})

		AfterEach(func() {
			deprovResp1 := brokerAPIClient.Deprovision(orphanInstanceGUID, brokerInfo.ServiceID, brokerInfo.PlanID)
			deprovResp2 := brokerAPIClient.Deprovision(nonOrphanInstanceGUID, brokerInfo.ServiceID, brokerInfo.PlanID)

			brokerAPIClient.PollLastOperation(orphanInstanceGUID, deprovResp1.OperationData)
			brokerAPIClient.PollLastOperation(nonOrphanInstanceGUID, deprovResp2.OperationData)
		})

		It("lists the orphan deployment", func() {
			orphanInstanceName := fmt.Sprintf("service-instance_%s", orphanInstanceGUID)
			nonOrphanInstanceName := fmt.Sprintf("service-instance_%s", nonOrphanInstanceGUID)

			By("setting up SI api to report only one instance")
			instancesToReturn := []service.Instance{
				{GUID: nonOrphanInstanceGUID},
			}
			err := siapi_helpers.UpdateServiceInstancesAPI(siapiConfig, instancesToReturn)
			Expect(err).ToNot(HaveOccurred())

			By("running the orphan-deployments errand")
			session := bosh.RunErrand(brokerInfo.DeploymentName, "orphan-deployments", gexec.Exit(1))

			By("checking the errand task output")
			Expect(session.ExitCode()).To(Equal(1))
			Expect(string(session.Buffer().Contents())).To(SatisfyAll(
				ContainSubstring("Orphan BOSH deployments detected"),
				ContainSubstring(`{"deployment_name":"%s"}`, orphanInstanceName),
				Not(ContainSubstring(nonOrphanInstanceName)),
			))
		})
	})
})
