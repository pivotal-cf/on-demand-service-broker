// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_registration_tests

import (
	"fmt"
	"github.com/pborman/uuid"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var (
	brokerInfo 	bosh.BrokerInfo
	cfAdminUsername          string
	cfAdminPassword          string
	cfSpaceDeveloperUsername string
	cfSpaceDeveloperPassword string
	cfDefaultSpaceDeveloperUsername string
	cfDefaultSpaceDeveloperPassword string
	cfOrg                    string
	defaultOrg                    string
	cfSpace                  string
	defaultSpace                  string
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]
	cfAdminUsername = envMustHave("CF_USERNAME")
	cfAdminPassword = envMustHave("CF_PASSWORD")
	cfSpaceDeveloperUsername = uuid.New()[:8]
	cfSpaceDeveloperPassword = uuid.New()[:8]
	cfDefaultSpaceDeveloperUsername = uuid.New()[:8]
	cfDefaultSpaceDeveloperPassword = uuid.New()[:8]
	cfOrg = envMustHave("CF_ORG")
	cfSpace = envMustHave("CF_SPACE")

	defaultOrg = "org-"+uniqueID
	defaultSpace = "space-"+uniqueID
	cfCreateDefaultOrgAndSpace()

	brokerInfo = bosh.DeployAndRegisterBroker(
		"-broker-registration-"+uniqueID,
		bosh.Redis,
		[]string{"update_service_catalog.yml", "update_default_access_org.yml"},
		"--var", "default_access_org=" + defaultOrg)

	SetDefaultEventuallyTimeout(cf.CfTimeout)
	cfCreateSpaceDevUser()
	cfCreateDefaultSpaceDevUser()
})

var _ = AfterSuite(func() {
	cfDeleteSpaceDevUser()
	cfDeleteDefaultOrg()
	bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
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
	cfLogInAsAdmin()
	Eventually(cf.Cf("create-user", cfSpaceDeveloperUsername, cfSpaceDeveloperPassword)).Should(gexec.Exit(0))
	Eventually(cf.Cf("set-space-role", cfSpaceDeveloperUsername, cfOrg, cfSpace, "SpaceDeveloper")).Should(gexec.Exit(0))
}

func cfCreateDefaultSpaceDevUser(){
	cfLogInAsAdmin()
	Eventually(cf.Cf("create-user", cfDefaultSpaceDeveloperUsername, cfDefaultSpaceDeveloperPassword)).Should(gexec.Exit(0))
	Eventually(cf.Cf("set-space-role", cfDefaultSpaceDeveloperUsername, defaultOrg, defaultSpace, "SpaceDeveloper")).Should(gexec.Exit(0))
}

func cfDeleteSpaceDevUser() {
	cfLogInAsAdmin()
	Eventually(cf.Cf("delete-user", cfSpaceDeveloperUsername, "-f")).Should(gexec.Exit(0))
}

func cfLogInAsSpaceDev() {
	cfLogout()
	Eventually(cf.Cf("auth", cfSpaceDeveloperUsername, cfSpaceDeveloperPassword)).Should(gexec.Exit(0))
	Eventually(cf.Cf("target", "-o", cfOrg, "-s", cfSpace)).Should(gexec.Exit(0))
}

func cfLogInAsDefaultSpaceDev() {
	cfLogout()
	Eventually(cf.Cf("auth", cfDefaultSpaceDeveloperUsername, cfDefaultSpaceDeveloperPassword)).Should(gexec.Exit(0))
	Eventually(cf.Cf("target", "-o", defaultOrg, "-s", defaultSpace)).Should(gexec.Exit(0))
}

func cfLogInAsAdmin() {
	cfLogout()
	Eventually(cf.Cf("auth", cfAdminUsername, cfAdminPassword)).Should(gexec.Exit(0))
	Eventually(cf.Cf("target", "-o", cfOrg, "-s", cfSpace)).Should(gexec.Exit(0))
}

func cfLogout() {
	Eventually(cf.Cf("logout")).Should(gexec.Exit(0))
}

func cfCreateDefaultOrgAndSpace() {
	cfLogInAsAdmin()
	Eventually(cf.Cf("create-org", defaultOrg)).Should(gexec.Exit(0))
	Eventually(cf.Cf("create-space", "-o", defaultOrg, defaultSpace)).Should(gexec.Exit(0))
}

func cfDeleteDefaultOrg() {
	cfLogInAsAdmin()
	Eventually(cf.Cf("delete-org", "-f", defaultOrg)).Should(gexec.Exit(0))
}

func targetSystemOrgAndSpace() {
	Eventually(cf.Cf("target", "-o", cfOrg, "-s", cfSpace)).Should(gexec.Exit(0))
}

func targetDefaultOrgAndSpace() {
	Eventually(cf.Cf("target", "-o", defaultOrg, "-s", defaultSpace)).Should(gexec.Exit(0))
}