// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_instances_errand_tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

var (
	brokerName                   string
	brokerUsername               string
	brokerPassword               string
	brokerURL                    string
	brokerBoshDeploymentName     string
	serviceOffering              string
	boshUsername                 string
	boshPassword                 string
	boshURL                      string
	boshCACert                   string
	originalBrokerManifest       *bosh.BoshManifest
	boshSupportsLifecycleErrands bool
	ciRootPath                   string
	exampleAppDirName            string
	boshClient                   *bosh_helpers.BoshHelperClient
	currentPlan                  string
	serviceGUID                  string
)

var _ = BeforeSuite(func() {
	brokerName = envMustHave("BROKER_NAME")
	brokerUsername = envMustHave("BROKER_USERNAME")
	brokerPassword = envMustHave("BROKER_PASSWORD")
	brokerURL = envMustHave("BROKER_URL")
	brokerBoshDeploymentName = envMustHave("BROKER_DEPLOYMENT_NAME")
	serviceOffering = envMustHave("SERVICE_NAME")

	boshURL = envMustHave("BOSH_URL")
	boshUsername = envMustHave("BOSH_USERNAME")
	boshPassword = envMustHave("BOSH_PASSWORD")

	uaaURL := os.Getenv("UAA_URL")
	boshCACert = os.Getenv("BOSH_CA_CERT_FILE")
	boshSupportsLifecycleErrands = os.Getenv("BOSH_SUPPORTS_LIFECYCLE_ERRANDS") == "true"

	ciRootPath = envMustHave("CI_ROOT_PATH")
	exampleAppDirName = envMustHave("EXAMPLE_APP_DIR_NAME")

	serviceGUID = envMustHave("SERVICE_GUID")

	if uaaURL == "" {
		boshClient = bosh_helpers.NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert, boshCACert == "")
	} else {
		boshClient = bosh_helpers.New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert)
	}

	originalBrokerManifest = boshClient.GetManifest(brokerBoshDeploymentName)
	Expect(originalBrokerManifest).NotTo(BeNil(),
		fmt.Sprintf("Deployment '%s' does not exist", brokerBoshDeploymentName))

	By("registering the broker")
	Eventually(cf.Cf("create-service-broker", brokerName, brokerUsername, brokerPassword, brokerURL), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	Eventually(cf.Cf("enable-service-access", serviceOffering), cf_helpers.CfTimeout).Should(gexec.Exit(0))
})

var _ = AfterSuite(func() {
	By("deregistering the broker")
	Eventually(cf.Cf("delete-service-broker", brokerName, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit(0))
})

func TestUpgradeInstancesErrandTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UpgradeInstancesErrand Suite")
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).NotTo(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}
