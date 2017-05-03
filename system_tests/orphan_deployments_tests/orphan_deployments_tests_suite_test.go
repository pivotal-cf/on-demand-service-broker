// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package orphan_deployments_tests

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
)

var (
	brokerName               string
	brokerBoshDeploymentName string
	serviceOffering          string
	boshClient               *bosh_helpers.BoshHelperClient
)

var _ = BeforeSuite(func() {
	brokerName = envMustHave("BROKER_NAME")
	brokerBoshDeploymentName = envMustHave("BROKER_DEPLOYMENT_NAME")
	serviceOffering = envMustHave("SERVICE_NAME")

	brokerURL := envMustHave("BROKER_URL")
	brokerUsername := envMustHave("BROKER_USERNAME")
	brokerPassword := envMustHave("BROKER_PASSWORD")
	uaaURL := os.Getenv("UAA_URL")
	boshURL := envMustHave("BOSH_URL")
	boshUsername := envMustHave("BOSH_USERNAME")
	boshPassword := envMustHave("BOSH_PASSWORD")
	boshCACert := os.Getenv("BOSH_CA_CERT_FILE")

	Eventually(cf.Cf("create-service-broker", brokerName, brokerUsername, brokerPassword, brokerURL), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	Eventually(cf.Cf("enable-service-access", serviceOffering), cf_helpers.CfTimeout).Should(gexec.Exit(0))

	if uaaURL == "" {
		boshClient = bosh_helpers.NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert, boshCACert == "")
	} else {
		boshClient = bosh_helpers.New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert)
	}
})

var _ = AfterSuite(func() {
	Eventually(cf.Cf("delete-service-broker", brokerName, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit(0))
})

func TestOrphanDeploymentsTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OrphanDeploymentsTests Suite")
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}
