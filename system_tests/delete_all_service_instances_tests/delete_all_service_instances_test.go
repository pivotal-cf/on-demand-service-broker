// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package delete_all_service_instances_tests

import (
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"

	. "github.com/onsi/ginkgo"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var (
	serviceInstanceNameOne string
	serviceInstanceNameTwo string
	serviceInstanceGuidOne string
	serviceInstanceGuidTwo string
	serviceKeyName         string
	appName                string
	appURL                 string
)

var _ = Describe("deleting all service instances", func() {

	BeforeEach(func() {
		serviceInstanceNameOne = "service-one-" + brokerInfo.TestSuffix
		serviceInstanceNameTwo = "service-two-" + brokerInfo.TestSuffix

		cf_helpers.CreateServiceWithoutWaiting(brokerInfo.ServiceOffering, "redis-small", serviceInstanceNameOne, "")
		cf_helpers.CreateServiceWithoutWaiting(brokerInfo.ServiceOffering, "redis-small", serviceInstanceNameTwo, "")

		cf_helpers.AwaitServiceCreation(serviceInstanceNameOne)
		cf_helpers.AwaitServiceCreation(serviceInstanceNameTwo)
	})

	AfterEach(func() {
		cf_helpers.DeleteApp(appName)
		cf_helpers.DeleteServiceKeyWithoutChecking(serviceInstanceNameOne, serviceKeyName)
		cf_helpers.DeleteServiceWithoutChecking(serviceInstanceNameOne)
		cf_helpers.DeleteServiceWithoutChecking(serviceInstanceNameTwo)
	})

	It("deletes and unbinds all service instances", func() {
		By("pushing and binding an app to Service Instance One", func() {
			appName = "example-app" + brokerInfo.TestSuffix
			appPath := cf_helpers.GetAppPath(service_helpers.Redis)
			appURL = cf_helpers.PushAndBindApp(appName, serviceInstanceNameOne, appPath)
		})

		By("creating a service key for Service Instance One", func() {
			serviceKeyName = "serviceKey" + brokerInfo.TestSuffix
			cf_helpers.CreateServiceKey(serviceInstanceNameOne, serviceKeyName)
			serviceKeyContents := cf_helpers.GetServiceKey(serviceInstanceNameOne, serviceKeyName)
			cf_helpers.LooksLikeAServiceKey(serviceKeyContents)
		})

		By("verifying secrets are stored in credhub", func() {
			serviceInstanceGuidOne = cf_helpers.GetServiceInstanceGUID(serviceInstanceNameOne)
			credhubCLI.VerifyCredhubKeysExist(brokerInfo.ServiceOffering, serviceInstanceGuidOne)

			serviceInstanceGuidTwo = cf_helpers.GetServiceInstanceGUID(serviceInstanceNameTwo)
			credhubCLI.VerifyCredhubKeysExist(brokerInfo.ServiceOffering, serviceInstanceGuidTwo)
		})

		By("running delete-all-service-instances errand", func() {
			bosh_helpers.RunErrand(brokerInfo.DeploymentName, "delete-all-service-instances")
			cf_helpers.AwaitServiceDeletion(serviceInstanceNameOne)
			cf_helpers.AwaitServiceDeletion(serviceInstanceNameTwo)
		})

		By("removing all credhub references relating to instances that existed when the errand was invoked", func() {
			credhubCLI.VerifyCredhubKeysEmpty(brokerInfo.ServiceOffering, serviceInstanceGuidOne)
			credhubCLI.VerifyCredhubKeysEmpty(brokerInfo.ServiceOffering, serviceInstanceGuidTwo)
		})
	})
})
