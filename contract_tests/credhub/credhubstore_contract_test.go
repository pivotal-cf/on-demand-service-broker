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

package credhub_tests

import (
	"encoding/json"
	"log"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	odbcredhub "github.com/pivotal-cf/on-demand-service-broker/credhub"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

var _ = Describe("Credential store", func() {

	var (
		subject       *odbcredhub.Store
		credhubClient *credhub.CredHub
	)

	BeforeEach(func() {
		subject = getCredhubStore()
		credhubClient = underlyingCredhubClient()
	})

	Describe("Set (and delete)", func() {
		It("sets and deletes a key-value map credential", func() {
			keyPath := makeKeyPath("new-name")
			err := subject.Set(keyPath, map[string]interface{}{"hi": "there"})
			Expect(err).NotTo(HaveOccurred())

			err = subject.Delete(keyPath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("can store plain string values", func() {
			keyPath := makeKeyPath("stringy-cred")
			err := subject.Set(keyPath, "I JUST LOVE CREDENTIALS.")
			Expect(err).NotTo(HaveOccurred())
		})

		It("produces error when storing other types", func() {
			keyPath := makeKeyPath("esoteric-cred")
			err := subject.Set(keyPath, []interface{}{"asdf"})
			Expect(err).To(MatchError("Unknown credential type"))
		})
	})

	Describe("BulkSet", func() {
		It("sets multiple values", func() {
			path1 := makeKeyPath("secret-1")
			path2 := makeKeyPath("secret-2")
			err := subject.BulkSet([]task.ManifestSecret{
				{Name: "secret-1", Path: path1, Value: map[string]interface{}{"hi": "there"}},
				{Name: "secret-2", Path: path2, Value: "value2"},
			})
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				credhubClient.Delete(path1)
				credhubClient.Delete(path2)
			}()

			cred1, err := credhubClient.GetLatestJSON(path1)
			Expect(err).NotTo(HaveOccurred(), path1)
			cred2, err := credhubClient.GetLatestValue(path2)
			Expect(err).NotTo(HaveOccurred(), path2)

			Expect(cred1.Value).To(Equal(values.JSON{"hi": "there"}))
			Expect(cred2.Value).To(Equal(values.Value("value2")))
		})
	})

	Describe("Add permission", func() {
		It("can add permissions", func() {
			keyPath := makeKeyPath("new-name")
			err := subject.Set(keyPath, map[string]interface{}{"hi": "there"})
			Expect(err).NotTo(HaveOccurred())

			_, err = subject.AddPermissions(keyPath, []permissions.Permission{
				{Actor: "alice", Operations: []string{"read"}},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Build", func() {
		It("can't be constructed with a bad URI", func() {
			_, err := odbcredhub.Build("💩://hi.there#you", credhub.SkipTLSValidation(true))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot contain colon"))
		})
	})

	Describe("BulkGet", func() {
		var (
			jsonSecret     credentials.JSON
			passwordSecret credentials.Password
		)

		BeforeEach(func() {
			var err error
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

			actualSecrets, err := subject.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))
		})

		It("should use ID when present", func() {
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

			actualSecrets, err := subject.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))

			By("fetching the secret by Path when Id isn't present")
			secretsToFetch = map[string]boshdirector.Variable{
				passwordSecret.Name: {Path: "foo"},
			}
			expectedSecrets = map[string]string{
				passwordSecret.Name: string(newPasswordSecret.Value),
			}
			actualSecrets, err = subject.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))
		})

		It("logs when the credential doesn't exist", func() {
			secretsToFetch := map[string]boshdirector.Variable{
				"blah": {Path: "blah"},
			}
			outputBuffer := gbytes.NewBuffer()
			logger := log.New(outputBuffer, "contract-tests", log.LstdFlags)
			_, err := subject.BulkGet(secretsToFetch, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(outputBuffer).To(gbytes.Say("Could not resolve blah"))
		})
	})
})
