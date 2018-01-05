// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_registration_tests

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
	serviceOffering          string
	brokerName               string
	brokerBoshDeploymentName string
	cfAdminUsername          string
	cfAdminPassword          string
	cfSpaceDeveloperUsername string
	cfSpaceDeveloperPassword string
	cfOrg                    string
	cfSpace                  string
	boshClient               *bosh_helpers.BoshHelperClient
)

var _ = BeforeSuite(func() {
	serviceOffering = envMustHave("SERVICE_NAME")
	brokerName = envMustHave("BROKER_NAME")
	brokerBoshDeploymentName = envMustHave("BROKER_DEPLOYMENT_NAME")

	boshURL := envMustHave("BOSH_URL")
	boshUsername := envMustHave("BOSH_USERNAME")
	boshPassword := envMustHave("BOSH_PASSWORD")
	boshCACert := os.Getenv("BOSH_CA_CERT_FILE")
	disableTLSVerification := boshCACert == ""
	uaaURL := os.Getenv("UAA_URL")

	cfAdminUsername = envMustHave("CF_USERNAME")
	cfAdminPassword = envMustHave("CF_PASSWORD")
	cfSpaceDeveloperUsername = envMustHave("CF_SPACE_DEVELOPER_USERNAME")
	cfSpaceDeveloperPassword = envMustHave("CF_SPACE_DEVELOPER_PASSWORD")
	cfOrg = envMustHave("CF_ORG")
	cfSpace = envMustHave("CF_SPACE")

	if uaaURL == "" {
		boshClient = bosh_helpers.NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert, disableTLSVerification)
	} else {
		boshClient = bosh_helpers.New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert)
	}
	SetDefaultEventuallyTimeout(cf_helpers.CfTimeout)
	cfCreateSpaceDevUser()
})

var _ = AfterSuite(func() {
	cfLogInAsAdmin()
	cfDeleteSpaceDevUser()
	Eventually(cf.Cf("delete-service-broker", brokerName, "-f")).Should(gexec.Exit(0))
	gexec.CleanupBuildArtifacts()
})

func TestMarketplaceTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Broker Registration Suite")
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}

func cfCreateSpaceDevUser() {
	Eventually(cf.Cf("create-user", cfSpaceDeveloperUsername, cfSpaceDeveloperPassword)).Should(gexec.Exit(0))
	Eventually(cf.Cf("set-space-role", cfSpaceDeveloperUsername, cfOrg, cfSpace, "SpaceDeveloper")).Should(gexec.Exit(0))
}

func cfDeleteSpaceDevUser() {
	Eventually(cf.Cf("delete-user", cfSpaceDeveloperUsername, "-f")).Should(gexec.Exit(0))
}

func cfLogInAsSpaceDev() {
	Eventually(cf.Cf("auth", cfSpaceDeveloperUsername, cfSpaceDeveloperPassword)).Should(gexec.Exit(0))
	Eventually(cf.Cf("target", "-o", cfOrg, "-s", cfSpace)).Should(gexec.Exit(0))
}

func cfLogInAsAdmin() {
	Eventually(cf.Cf("auth", cfAdminUsername, cfAdminPassword)).Should(gexec.Exit(0))
	Eventually(cf.Cf("target", "-o", cfOrg, "-s", cfSpace)).Should(gexec.Exit(0))
}
