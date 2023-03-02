// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_registration_tests

import (
	"fmt"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"

	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var (
	brokerInfo                      bosh.BrokerInfo
	cfAdminUsername                 string
	cfAdminPassword                 string
	cfSpaceDeveloperUsername        string
	cfSpaceDeveloperPassword        string
	cfDefaultSpaceDeveloperUsername string
	cfDefaultSpaceDeveloperPassword string
	cfOrg                           string
	defaultOrg                      string
	cfSpace                         string
	defaultSpace                    string
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

	defaultOrg = "org-" + uniqueID
	defaultSpace = "space-" + uniqueID
	cfCreateDefaultOrgAndSpace()

	brokerInfo = bosh.DeployAndRegisterBroker(
		"-broker-registration-"+uniqueID,
		bosh.BrokerDeploymentOptions{},
		service_helpers.Redis,
		[]string{"update_service_catalog.yml"},
		"--var", "org="+defaultOrg)

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
	Expect(cf.Cf("create-user", cfSpaceDeveloperUsername, cfSpaceDeveloperPassword)).To(gexec.Exit(0))
	Expect(cf.Cf("set-space-role", cfSpaceDeveloperUsername, cfOrg, cfSpace, "SpaceDeveloper")).To(gexec.Exit(0))
}

func cfCreateDefaultSpaceDevUser() {
	cfLogInAsAdmin()
	Expect(cf.Cf("create-user", cfDefaultSpaceDeveloperUsername, cfDefaultSpaceDeveloperPassword)).To(gexec.Exit(0))
	Expect(cf.Cf("set-space-role", cfDefaultSpaceDeveloperUsername, defaultOrg, defaultSpace, "SpaceDeveloper")).To(gexec.Exit(0))
}

func cfDeleteSpaceDevUser() {
	cfLogInAsAdmin()
	Expect(cf.Cf("delete-user", cfSpaceDeveloperUsername, "-f")).To(gexec.Exit(0))
}

func cfLogInAsSpaceDev() {
	cfLogout()
	Expect(cf.Cf("auth", cfSpaceDeveloperUsername, cfSpaceDeveloperPassword)).To(gexec.Exit(0))
	Expect(cf.Cf("target", "-o", cfOrg, "-s", cfSpace)).To(gexec.Exit(0))
}

func cfLogInAsDefaultSpaceDev() {
	cfLogout()
	Expect(cf.Cf("auth", cfDefaultSpaceDeveloperUsername, cfDefaultSpaceDeveloperPassword)).To(gexec.Exit(0))
	Expect(cf.Cf("target", "-o", defaultOrg, "-s", defaultSpace)).To(gexec.Exit(0))
}

func cfLogInAsAdmin() {
	cfLogout()
	Expect(cf.Cf("auth", cfAdminUsername, cfAdminPassword)).To(gexec.Exit(0))
	Expect(cf.Cf("target", "-o", cfOrg, "-s", cfSpace)).To(gexec.Exit(0))
}

func cfLogout() {
	Expect(cf.Cf("logout")).To(gexec.Exit(0))
}

func cfCreateDefaultOrgAndSpace() {
	cfLogInAsAdmin()
	Expect(cf.Cf("create-org", defaultOrg)).To(gexec.Exit(0))
	Expect(cf.Cf("create-space", "-o", defaultOrg, defaultSpace)).To(gexec.Exit(0))
}

func cfDeleteDefaultOrg() {
	cfLogInAsAdmin()
	Expect(cf.Cf("delete-org", "-f", defaultOrg)).To(gexec.Exit(0))
}

func targetSystemOrgAndSpace() {
	Expect(cf.Cf("target", "-o", cfOrg, "-s", cfSpace)).To(gexec.Exit(0))
}

func targetDefaultOrgAndSpace() {
	Expect(cf.Cf("target", "-o", defaultOrg, "-s", defaultSpace)).To(gexec.Exit(0))
}
