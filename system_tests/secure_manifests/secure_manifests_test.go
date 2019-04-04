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

package secure_manifests_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("Secure Manifests", func() {

	var (
		serviceInstanceName string
		serviceInstanceGUID string
		serviceKeyName      string
		adapterSecretValue  string
		plan                string
		secretKey           string
	)

	BeforeEach(func() {
		plan = "redis-small"
		serviceInstanceName = "service" + brokerInfo.TestSuffix

		secretKey = "odb_managed_secret"
		adapterSecretValue = "HardcodedAdapterValue"
	})

	It("replaces plain text secrets with CredHub references in the manifest", func() {
		By("creating a service with ODB managed secrets", func() {
			cf_helpers.CreateService(brokerInfo.ServiceOffering, plan, serviceInstanceName, "")
		})

		By("downloading the manifest and confirming it has a CredHub path instead of the plain text secret", func() {
			serviceInstanceGUID = cf_helpers.GetServiceInstanceGUID(serviceInstanceName)
			serviceDeploymentName := broker.InstancePrefix + serviceInstanceGUID
			manifest := bosh_helpers.GetManifestString(serviceDeploymentName)
			Expect(manifest).To(SatisfyAll(
				ContainSubstring("/odb/%s/%s/%s", brokerInfo.ServiceOffering, serviceDeploymentName, secretKey),
				Not(ContainSubstring(adapterSecretValue)),
			))
		})

		By("verifying the secret value exists on CredHub", func() {
			expectTheSecretValueToBeInCredhub(serviceInstanceGUID, secretKey, adapterSecretValue)
		})

		var serviceKeyContents string

		By("creating a service key", func() {
			serviceKeyName = "serviceKey" + brokerInfo.TestSuffix
			cf_helpers.CreateServiceKey(serviceInstanceName, serviceKeyName)
			serviceKeyContents = cf_helpers.GetServiceKey(serviceInstanceName, serviceKeyName)
		})

		By("verifying that the secret value is passed to the adapter on create-binding", func() {
			odbManagedSecret := getODBManagedSecret(serviceKeyContents)
			Expect(odbManagedSecret).To(Equal(adapterSecretValue))
		})

		By("verifying that the secret value is passed to the adapter on delete-binding", func() {
			cf_helpers.DeleteServiceKey(serviceInstanceName, serviceKeyName)
		})

		By("updating the service instance with a new value for the secret", func() {
			cf_helpers.UpdateServiceWithArbitraryParams(serviceInstanceName, `{ "odb_managed_secret": "new-value" }`)
		})

		By("verifying the secret value is updated on CredHub", func() {
			expectTheSecretValueToBeInCredhub(serviceInstanceGUID, secretKey, "new-value")
		})
	})
})

func expectTheSecretValueToBeInCredhub(serviceInstanceGUID, secretKey, adapterSecretValue string) {
	odbSecret := credhubCLI.GetCredhubValueFor(brokerInfo.ServiceOffering, serviceInstanceGUID, secretKey)
	Expect(odbSecret["value"]).To(Equal(adapterSecretValue))
}

func getODBManagedSecret(serviceKeyContents string) string {
	var serviceKey struct {
		ODBManagedSecret string `json:"odb_managed_secret"`
	}
	err := json.Unmarshal([]byte(serviceKeyContents), &serviceKey)
	Expect(err).ToNot(HaveOccurred())
	return serviceKey.ODBManagedSecret
}
