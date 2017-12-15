// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package lifecycle_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var (
	brokerName           string
	brokerDeploymentName string
	brokerURL            string
	brokerUsername       string
	brokerPassword       string
	serviceOffering      string
	serviceID            string
	dopplerAddress       string
	exampleAppPath       string
	exampleAppType       string
	tests                = parseTests()
	shouldTestODBMetrics bool
	shouldTestCredhubRef bool
)

func parseTests() []LifecycleTest {
	lifecycleConfig := os.Getenv("LIFECYCLE_TESTS_CONFIG")
	if lifecycleConfig == "" {
		panic("must set $LIFECYCLE_TESTS_CONFIG")
	}
	lifecycleConfigFile, err := os.Open(lifecycleConfig)
	if err != nil {
		panic(err)
	}
	defer lifecycleConfigFile.Close()
	var tests []LifecycleTest
	if err := json.NewDecoder(lifecycleConfigFile).Decode(&tests); err != nil {
		panic(err)
	}
	if len(tests) == 0 {
		panic("expected tests not to be empty")
	}
	return tests
}

type LifecycleTest struct {
	Plan            string          `json:"plan"`
	UpdateToPlan    string          `json:"update_to_plan"`
	ArbitraryParams json.RawMessage `json:"arbitrary_params"`
}

var _ = SynchronizedBeforeSuite(func() []byte {
	parseEnv()
	Eventually(cf.Cf("create-service-broker", brokerName, brokerUsername, brokerPassword, brokerURL), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	Eventually(cf.Cf("enable-service-access", serviceOffering), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	return []byte{}
}, func(data []byte) {
	parseEnv()
})

func parseEnv() {
	brokerName = envMustHave("BROKER_NAME")
	brokerDeploymentName = envMustHave("BROKER_DEPLOYMENT_NAME")
	brokerUsername = envMustHave("BROKER_USERNAME")
	brokerPassword = envMustHave("BROKER_PASSWORD")
	brokerURL = envMustHave("BROKER_URL")
	serviceOffering = envMustHave("SERVICE_NAME")
	serviceID = envMustHave("SERVICE_GUID")
	dopplerAddress = os.Getenv("DOPPLER_ADDRESS")
	exampleAppPath = envMustHave("EXAMPLE_APP_PATH")
	exampleAppType = envMustHave("EXAMPLE_APP_TYPE")
	shouldTestODBMetrics = os.Getenv("TEST_ODB_METRICS") != ""
	shouldTestCredhubRef = os.Getenv("TEST_CREDHUB_REF") == "true"
}

var _ = SynchronizedAfterSuite(func() {}, func() {
	session := cf.Cf("services")
	Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit(0))
	services := parseServiceNames(string(session.Buffer().Contents()))

	for _, service := range services {
		Eventually(cf.Cf("delete-service", service, "-f"), time.Minute).Should(gexec.Exit(0))
	}

	for _, service := range services {
		Eventually(func() bool {
			session := cf.Cf("service", service)
			Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())
			if session.ExitCode() != 0 {
				return true
			} else {
				time.Sleep(time.Second * 5)
				return false
			}
		}, cf_helpers.LongCfTimeout).Should(BeTrue())
	}

	Eventually(cf.Cf("delete-service-broker", brokerName, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit(0))
})

func parseServiceNames(cfServicesOutput string) []string {
	services := []string{}
	for _, line := range strings.Split(cfServicesOutput, "\n") {
		if strings.Contains(line, serviceOffering) {
			services = append(services, strings.Fields(line)[0])
		}
	}
	return services
}

func TestSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lifecycle Suite")
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}
