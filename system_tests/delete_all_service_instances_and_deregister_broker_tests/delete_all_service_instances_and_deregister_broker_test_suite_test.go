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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var (
	exampleAppPath string
	boshClient     *bosh_helpers.BoshHelperClient
	brokerInfo     bosh_helpers.BrokerInfo
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]
	brokerInfo = bosh_helpers.DeployAndRegisterBroker("-delete-all-"+uniqueID, []string{"update_service_catalog.yml"})

	boshURL := envMustHave("BOSH_URL")
	boshUsername := envMustHave("BOSH_USERNAME")
	boshPassword := envMustHave("BOSH_PASSWORD")
	uaaURL := os.Getenv("UAA_URL")
	boshCACert := os.Getenv("BOSH_CA_CERT_FILE")
	disableTLSVerification := boshCACert == ""
	exampleAppPath = envMustHave("EXAMPLE_APP_PATH")

	if uaaURL == "" {
		boshClient = bosh_helpers.NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert, disableTLSVerification)
	} else {
		boshClient = bosh_helpers.New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert)
	}
})

var _ = AfterSuite(func() {
	bosh_helpers.RunErrand(brokerInfo.DeploymentName, "delete-all-service-instances")
	session := cf.Cf("delete-service-broker", "-f", brokerInfo.ServiceOffering)
	Eventually(session, cf.CfTimeout).Should(gexec.Exit())
	boshClient.DeleteDeployment(brokerInfo.DeploymentName)
	gexec.CleanupBuildArtifacts()
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
