// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package credstore_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/credstore"
	"github.com/pivotal-cf/on-demand-service-broker/mockcredhub"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("Credhub Client", func() {
	var (
		credhubUaa          *mockuaa.UserCredentialsServer
		credhub             *mockhttp.Server
		credhubClientID     = "credhubAdminUsername"
		credhubClientSecret = "credhubAdminPassword"
		identifier          = "identifier"
	)

	BeforeEach(func() {
		credhubUaa = mockuaa.NewUserCredentialsServer("credhub", "", credhubClientID, credhubClientSecret, "Credhub token")
		credhub = mockcredhub.New()
	})

	AfterEach(func() {
		credhub.VerifyMocks()
		credhub.Close()
		credhubUaa.Close()
	})

	Context("when putting the credential succeeds", func() {
		BeforeEach(func() {
			secret := `{"secret":"things"}`
			credhub.VerifyAndMock(
				mockcredhub.GetInfo().RespondsWithCredhubUaaUrl(credhubUaa.URL),
				mockcredhub.PutCredential(identifier).WithPassword(secret).RespondsWithPasswordData(secret),
			)
		})

		It("does not return an error", func() {
			client := credstore.NewCredhubClient(credhub.URL, credhubClientID, credhubClientSecret, true)

			err := client.PutCredentials(identifier, map[string]interface{}{"secret": "things"})

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when the credentials are invalid JSON", func() {
		It("returns an error", func() {
			client := credstore.NewCredhubClient(credhub.URL, credhubClientID, credhubClientSecret, true)
			invalidCredentialsMap := map[string]interface{}{
				"key": map[float64]string{
					1.0: "thing",
				},
			}

			err := client.PutCredentials(identifier, invalidCredentialsMap)

			Expect(err).To(MatchError("error marshalling credentials"))
		})
	})

	Context("when the Credhub client credentials are unauthorized", func() {
		BeforeEach(func() {
			credhub.VerifyAndMock(
				mockcredhub.GetInfo().RespondsWithCredhubUaaUrl(credhubUaa.URL),
			)
		})

		It("returns an error", func() {
			client := credstore.NewCredhubClient(credhub.URL, credhubClientID, "not the client secret", true)

			err := client.PutCredentials(identifier, map[string]interface{}{"secret": "things"})

			Expect(err).To(MatchError("error getting credhub auth token"))
		})
	})

	Context("when put credential request to Credhub fails", func() {
		BeforeEach(func() {
			credhub.VerifyAndMock(
				mockcredhub.GetInfo().RespondsWithCredhubUaaUrl(credhubUaa.URL),
				mockcredhub.PutCredential(identifier).Fails("Credhub put credential error"),
			)
		})

		It("returns an error", func() {
			client := credstore.NewCredhubClient(credhub.URL, credhubClientID, credhubClientSecret, true)

			err := client.PutCredentials(identifier, map[string]interface{}{"secret": "things"})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error putting password into credhub"))
		})
	})
})
