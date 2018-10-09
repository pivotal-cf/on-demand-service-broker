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
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/siapi_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all/shared"
)

var (
	brokerName               string
	brokerBoshDeploymentName string
	serviceOffering          string
	boshClient               *bosh_helpers.BoshHelperClient
	siapiConfig              siapi_helpers.SIAPIConfig
)

var _ = BeforeSuite(func() {
	brokerName = shared.EnvMustHave("BROKER_NAME")
	brokerBoshDeploymentName = shared.EnvMustHave("BROKER_DEPLOYMENT_NAME")
	serviceOffering = shared.EnvMustHave("SERVICE_OFFERING_NAME")

	brokerURL := shared.EnvMustHave("BROKER_URL")
	brokerUsername := shared.EnvMustHave("BROKER_USERNAME")
	brokerPassword := shared.EnvMustHave("BROKER_PASSWORD")
	uaaURL := os.Getenv("UAA_URL")
	boshURL := shared.EnvMustHave("BOSH_URL")
	boshUsername := shared.EnvMustHave("BOSH_USERNAME")
	boshPassword := shared.EnvMustHave("BOSH_PASSWORD")
	boshCACert := os.Getenv("BOSH_CA_CERT_FILE")

	Eventually(cf.Cf("create-service-broker", brokerName, brokerUsername, brokerPassword, brokerURL), cf.CfTimeout).Should(gexec.Exit(0))
	Eventually(cf.Cf("enable-service-access", serviceOffering), cf.CfTimeout).Should(gexec.Exit(0))

	if uaaURL == "" {
		boshClient = bosh_helpers.NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert, boshCACert == "")
	} else {
		boshClient = bosh_helpers.New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert)
	}

	siapiConfig = siapi_helpers.SIAPIConfig{
		URL:      shared.EnvMustHave("SIAPI_URL"),
		Password: shared.EnvMustHave("SIAPI_PASSWORD"),
		Username: shared.EnvMustHave("SIAPI_USERNAME"),
	}
})

var _ = AfterSuite(func() {
	Eventually(cf.Cf("delete-service-broker", brokerName, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
})

func TestOrphanDeploymentsTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Orphan Deployments Errand With SIAPI Test Suite")
}
