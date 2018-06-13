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

package credstore_test

import (
	"encoding/json"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/credstore"
)

var _ = Describe("Operations", func() {
	Describe("BulkGet", func() {
		var credhubClient *credhub.CredHub
		var jsonSecret credentials.JSON
		var passwordSecret credentials.Password

		BeforeEach(func() {
			var err error
			credhubClient = credhubCorrectAuth()
			passwordSecret, err = credhubClient.SetPassword("foo", "thepass", "overwrite")
			Expect(err).NotTo(HaveOccurred())
			jsonSecret, err = credhubClient.SetJSON("jsonsecret", map[string]interface{}{"value": "foo"}, "overwrite")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(credhubClient.Delete(passwordSecret.Name)).To(Succeed())
			Expect(credhubClient.Delete(jsonSecret.Name)).To(Succeed())
		})

		It("can fetch secrets from credhub", func() {
			b := credstore.New(credhubClient)
			secretsToFetch := map[string]boshdirector.Variable{
				passwordSecret.Name: {Path: passwordSecret.Name},
				jsonSecret.Name:     {Path: jsonSecret.Name, ID: jsonSecret.Id},
			}
			jsonSecretValue, err := json.Marshal(jsonSecret.Value)
			Expect(err).NotTo(HaveOccurred())
			expectedSecrets := map[string]string{
				passwordSecret.Name: string(passwordSecret.Value),
				jsonSecret.Name:     string(jsonSecretValue),
			}

			actualSecrets, err := b.BulkGet(secretsToFetch)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))
		})

		It("should use ID when present", func() {
			b := credstore.New(credhubClient)

			By("creating two versions of the same secret")
			newPasswordSecret, err := credhubClient.SetPassword(passwordSecret.Name, "newthepass", "overwrite")
			Expect(err).NotTo(HaveOccurred())

			By("fetching the secret by ID when present")
			secretsToFetch := map[string]boshdirector.Variable{
				passwordSecret.Name: {Path: "foo", ID: passwordSecret.Id},
			}
			expectedSecrets := map[string]string{
				passwordSecret.Name: string(passwordSecret.Value),
			}

			actualSecrets, err := b.BulkGet(secretsToFetch)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))

			By("fetching the secret by Path when Id isn't present")
			secretsToFetch = map[string]boshdirector.Variable{
				passwordSecret.Name: {Path: "foo"},
			}
			expectedSecrets = map[string]string{
				passwordSecret.Name: string(newPasswordSecret.Value),
			}
			actualSecrets, err = b.BulkGet(secretsToFetch)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))
		})

		It("errors when the credential don't exist", func() {
			b := credstore.New(credhubClient)
			secretsToFetch := map[string]boshdirector.Variable{
				"blah": {Path: "blah"},
			}
			_, err := b.BulkGet(secretsToFetch)
			Expect(err).To(MatchError(ContainSubstring("credential does not exist")))
		})
	})
})
