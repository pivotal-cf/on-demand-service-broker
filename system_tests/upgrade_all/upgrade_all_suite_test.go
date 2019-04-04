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

package upgrade_all_test

import (
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

type appDetails struct {
	uuid                  string
	appURL                string
	appName               string
	serviceName           string
	serviceGUID           string
	serviceDeploymentName string
}

func TestUpgradeInstancesErrandTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UpgradeInstancesErrand Suite")
}

func performInParallel(f func(), count int) {
	var wg sync.WaitGroup
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			f()
		}()
	}

	wg.Wait()
}

func deployService(serviceOffering, planName, appPath string) appDetails {
	uuid := uuid.New()[:8]
	serviceName := "service-" + uuid
	appName := "app-" + uuid
	cf_helpers.CreateService(serviceOffering, planName, serviceName, "")
	serviceGUID := cf_helpers.ServiceInstanceGUID(serviceName)
	appURL := cf_helpers.PushAndBindApp(appName, serviceName, appPath)
	cf_helpers.PutToTestApp(appURL, "uuid", uuid)

	return appDetails{
		uuid:                  uuid,
		appURL:                appURL,
		appName:               appName,
		serviceName:           serviceName,
		serviceGUID:           serviceGUID,
		serviceDeploymentName: "service-instance_" + serviceGUID,
	}
}
