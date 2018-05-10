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
	"os"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
)

var _ = Describe("Credential store", func() {
	It("sets and deletes a key-value map credential", func() {
		keyPath := makeKeyPath("new-name")
		correctAuth := credhubCorrectAuth()
		err := correctAuth.Set(keyPath, map[string]interface{}{"hi": "there"})
		Expect(err).NotTo(HaveOccurred())

		err = correctAuth.Delete(keyPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("can store plain string values", func() {
		keyPath := makeKeyPath("stringy-cred")
		correctAuth := credhubCorrectAuth()
		err := correctAuth.Set(keyPath, "I JUST LOVE CREDENTIALS.")
		Expect(err).NotTo(HaveOccurred())
	})

	It("produces error when storing other types", func() {
		keyPath := makeKeyPath("esoteric-cred")
		correctAuth := credhubCorrectAuth()
		err := correctAuth.Set(keyPath, []interface{}{"asdf"})
		Expect(err).To(MatchError("Unknown credential type"))
	})

	It("produces error when authenticating late without UAA config", func() {
		keyPath := makeKeyPath("doesnt-really-matter")
		noUAAConfig := credhubNoUAAConfig()
		err := noUAAConfig.Delete(keyPath)
		Expect(err.Error()).To(ContainSubstring("invalid_token"))
	})

	It("produces error when authenticating early without UAA config", func() {
		noUAAConfig := credhubNoUAAConfig()
		err := noUAAConfig.Authenticate()
		Expect(err).To(HaveOccurred())
	})

	It("produces error with incorrect credentials", func() {
		incorrectAuth := credhubIncorrectAuth()
		err := incorrectAuth.Authenticate()
		Expect(err).To(HaveOccurred())
	})

	Describe("CredHub credential store", func() {
		It("can add permissions", func() {
			clientSecret := os.Getenv("TEST_CREDHUB_CLIENT_SECRET")

			credhubStore, err := credhubbroker.NewCredHubStore(
				"https://credhub.service.cf.internal:8844",
				credhub.SkipTLSValidation(true),
				credhub.Auth(auth.UaaClientCredentials("credhub_cli", clientSecret)),
			)
			Expect(err).NotTo(HaveOccurred())

			keyPath := makeKeyPath("new-name")
			err = credhubStore.Set(keyPath, map[string]interface{}{"hi": "there"})
			Expect(err).NotTo(HaveOccurred())

			_, err = credhubStore.AddPermissions(keyPath, []permissions.Permission{
				{Actor: "alice",
					Operations: []string{"read"},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("can't be constructed with a bad URI", func() {
			_, err := credhubbroker.NewCredHubStore("💩://hi.there#you", credhub.SkipTLSValidation(true))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot contain colon"))
		})
	})
})
