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

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/siapi_helpers"
)

var (
	siapiConfig siapi_helpers.SIAPIConfig
	appName     string
	brokerInfo  bosh.BrokerInfo
)

var _ = BeforeSuite(func() {

	uniqueID := uuid.New()[:6]

	appName = "si-api-" + uniqueID
	siAPIURL := "https://" + appName + "." + os.Getenv("BROKER_SYSTEM_DOMAIN") + "/service_instances"
	siAPIUsername := "siapi"
	siAPIPassword := "siapipass"

	cf.Cf("push",
		"-p", os.Getenv("SI_API_PATH"),
		"-f", os.Getenv("SI_API_PATH")+"/manifest.yml",
		"--var", "app_name="+appName,
		"--var", "username="+siAPIUsername,
		"--var", "password="+siAPIPassword,
	)

	brokerInfo = bosh.DeployBroker(
		"-orphan-deployment-with-siapi-"+uniqueID,
		bosh.BrokerDeploymentOptions{},
		service_helpers.Redis,
		[]string{"update_service_catalog.yml", "add_si_api.yml"},
		"--var", "service_instances_api_url="+siAPIURL,
		"--var", "service_instances_api_username="+siAPIUsername,
		"--var", "service_instances_api_password="+siAPIPassword,
	)

	siapiConfig = siapi_helpers.SIAPIConfig{
		URL:      siAPIURL,
		Username: siAPIUsername,
		Password: siAPIPassword,
	}
})

var _ = AfterSuite(func() {
	cf.Cf("delete", "-f", appName)
	bosh.DeleteDeployment(brokerInfo.DeploymentName)
})

func TestOrphanDeploymentsTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Orphan Deployments Errand With SIAPI Test Suite")
}
