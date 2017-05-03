// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package authorizationheader_test

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("Client Token Auth Header Builder", func() {
	var (
		mockUAA              *mockuaa.ClientCredentialsServer
		authorizer           *authorizationheader.ClientTokenAuthHeaderBuilder
		actualClientID       = "some-username"
		actualClientSecret   = "some-password"
		suppliedClientID     string
		suppliedClientSecret string
		tokenToReturn        = "some-token"
		buildErr             error
		header               string
	)

	BeforeEach(func() {
		mockUAA = mockuaa.NewClientCredentialsServer(actualClientID, actualClientSecret, tokenToReturn)
		mockUAA.ValiditySecondsToReturn = int(authorizationheader.MinimumRemainingValidity.Seconds()) + 1
	})

	AfterEach(func() {
		mockUAA.Close()
	})

	JustBeforeEach(func() {
		var certPEM []byte
		if mockUAA.TLS != nil {
			cert, err := x509.ParseCertificate(mockUAA.TLS.Certificates[0].Certificate[0])
			Expect(err).NotTo(HaveOccurred())
			certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		}
		var err error
		authorizer, err = authorizationheader.NewClientTokenAuthHeaderBuilder(mockUAA.URL, suppliedClientID, suppliedClientSecret, false, certPEM)
		Expect(err).NotTo(HaveOccurred())
		header, buildErr = authorizer.Build(logger)
	})

	Context("valid clientID and secret", func() {
		BeforeEach(func() {
			suppliedClientID = actualClientID
			suppliedClientSecret = actualClientSecret
		})

		It("succeeds", func() {
			Expect(buildErr).NotTo(HaveOccurred())
		})

		It("gets a token from UAA", func() {
			Expect(header).To(Equal(fmt.Sprintf("Bearer %s", tokenToReturn)))
		})

		It("obtains exactly one token from the UAA", func() {
			Expect(mockUAA.TokensIssued).To(Equal(1))
		})

		Context("when the UAA server is using HTTPS with a self-signed cert", func() {
			BeforeEach(func() {
				mockUAA.Close()
				mockUAA = mockuaa.NewClientCredentialsServerTLS(actualClientID, actualClientSecret, tokenToReturn)
			})

			It("succeeds", func() {
				Expect(buildErr).NotTo(HaveOccurred())
			})
		})

		Context("when a second authorization header is built", func() {
			var (
				secondHeader string
				secondErr    error
			)

			JustBeforeEach(func() {
				secondHeader, secondErr = authorizer.Build(logger)
			})

			It("succeeds", func() {
				Expect(secondErr).NotTo(HaveOccurred())
			})

			It("builds the same header", func() {
				Expect(secondHeader).To(Equal(header))
			})

			It("caches the first token and does not obtain a 2nd one from UAA", func() {
				Expect(mockUAA.TokensIssued).To(Equal(1))
			})

			Context("when the cached token has expired", func() {
				JustBeforeEach(func() {
					time.Sleep(time.Second * 2)
				})

				Context("when a third authorization header is built", func() {
					JustBeforeEach(func() {
						_, err := authorizer.Build(logger)
						Expect(err).ToNot(HaveOccurred())
					})

					It("obtains a new token from UAA", func() {
						Expect(mockUAA.TokensIssued).To(Equal(2))
					})
				})
			})
		})
	})

	Context("invalid clientID and secret", func() {
		BeforeEach(func() {
			suppliedClientID = "wrong"
			suppliedClientSecret = "wronger"
		})

		It("fails", func() {
			Expect(buildErr).To(MatchError(ContainSubstring(mockuaa.UnauthorizedError)))
			Expect(buildErr).To(MatchError(ContainSubstring(mockuaa.UnauthorizedErrorDescription)))
		})
	})

	Context("malformed response", func() {
		BeforeEach(func() {
			suppliedClientID = mockuaa.MalformedResponseUser
			suppliedClientSecret = ""
		})

		It("fails", func() {
			Expect(buildErr).To(MatchError(ContainSubstring("no access token in grant")))
		})
	})

	Context("uaa 500", func() {
		BeforeEach(func() {
			suppliedClientID = mockuaa.InternalServerErrorUser
			suppliedClientSecret = ""
		})

		It("fails", func() {
			Expect(buildErr).To(MatchError(ContainSubstring(mockuaa.InternalServerErrorMessage)))
		})
	})
})
