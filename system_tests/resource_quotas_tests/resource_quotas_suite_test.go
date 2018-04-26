// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package resource_quotas_tests

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

var (
	brokerName               string
	brokerUsername           string
	brokerPassword           string
	brokerURL                string
	brokerBoshDeploymentName string
	serviceOffering          string
	skipRedeploy             bool
	originalBrokerManifest   *bosh.BoshManifest
	boshClient               *bosh_helpers.BoshHelperClient
)

var _ = BeforeSuite(func() {
	brokerName = envMustHave("BROKER_NAME")
	brokerUsername = envMustHave("BROKER_USERNAME")
	brokerPassword = envMustHave("BROKER_PASSWORD")
	brokerURL = envMustHave("BROKER_URL")
	brokerBoshDeploymentName = envMustHave("BROKER_DEPLOYMENT_NAME")
	serviceOffering = envMustHave("SERVICE_NAME")

	boshURL := envMustHave("BOSH_URL")
	boshUsername := envMustHave("BOSH_USERNAME")
	boshPassword := envMustHave("BOSH_PASSWORD")
	uaaURL := os.Getenv("UAA_URL")
	boshCACert := os.Getenv("BOSH_CA_CERT_FILE")
	skipRedeploy = os.Getenv("SKIP_REDEPLOY") == "true"

	if uaaURL == "" {
		boshClient = bosh_helpers.NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert, boshCACert == "")
	} else {
		boshClient = bosh_helpers.New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert)
	}

	originalBrokerManifest = boshClient.GetManifest(brokerBoshDeploymentName)

	By("registering the broker")
	Eventually(cf.Cf("create-service-broker", brokerName, brokerUsername, brokerPassword, brokerURL), cf.CfTimeout).Should(gexec.Exit(0))
	Eventually(cf.Cf("enable-service-access", serviceOffering), cf.CfTimeout).Should(gexec.Exit(0))

	By("adding plan quotas to broker manifest")
	newBrokerManifest := boshClient.GetManifest(brokerBoshDeploymentName)
	brokerJob := bosh_helpers.FindJob(newBrokerManifest, "broker", "broker")

	serviceCatalog := brokerJob.Properties["service_catalog"].(map[interface{}]interface{})
	serviceCatalog["global_quotas"] = map[interface{}]interface{}{"resource_limits": map[string]int{"ips": 1}}

	dedicatedVMPlan := serviceCatalog["plans"].([]interface{})[0].(map[interface{}]interface{})
	dedicatedVMPlan["resource_costs"] = map[interface{}]interface{}{"ips": 1}

	By("deploying the modified broker manifest")
	boshClient.DeployODB(*newBrokerManifest)
})

var _ = AfterSuite(func() {
	By("deregistering the broker")
	Eventually(cf.Cf("delete-service-broker", brokerName, "-f"), cf.CfTimeout).Should(gexec.Exit(0))

	if !skipRedeploy {
		//cleanup for when running locally
		By("deploying the original broker manifest")
		boshClient.DeployODB(*originalBrokerManifest)
	}
})

func TestQuotasTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Quotas Suite")
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).NotTo(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}
