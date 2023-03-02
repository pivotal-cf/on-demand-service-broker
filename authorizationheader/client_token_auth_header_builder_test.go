// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package authorizationheader_test

import (
	"net/http"

	"crypto/x509"
	"encoding/pem"

	"fmt"

	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("Client Token Auth Header Builder", func() {
	var (
		mockUAA              *mockuaa.ClientCredentialsServer
		req                  *http.Request
		actualClientID       = "some-username"
		actualClientSecret   = "some-password"
		suppliedClientID     string
		suppliedClientSecret string
		tokenToReturn        = "some-token"
	)

	BeforeEach(func() {
		mockUAA = mockuaa.NewClientCredentialsServer(actualClientID, actualClientSecret, tokenToReturn)
		mockUAA.ValiditySecondsToReturn = int(authorizationheader.MinimumRemainingValidity.Seconds()) + 1
		suppliedClientID = actualClientID
		suppliedClientSecret = actualClientSecret
		var err error
		req, err = http.NewRequest("GET", "some-url-to-authorize", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		mockUAA.Close()
	})

	It("gets exactly one token from UAA", func() {
		authorizer := createClientTokenAuthorizer(mockUAA, suppliedClientID, suppliedClientSecret)
		err := authorizer.AddAuthHeader(req, logger)
		Expect(err).NotTo(HaveOccurred())

		authHeader := req.Header.Get("Authorization")
		Expect(authHeader).To(Equal(fmt.Sprintf("Bearer %s", tokenToReturn)))
		Expect(mockUAA.TokensIssued).To(Equal(1))
	})

	It("succeeds when the UAA server is using HTTPS with a self-signed cert", func() {
		mockUAA.Close()
		mockUAA = mockuaa.NewClientCredentialsServerTLS(actualClientID, actualClientSecret, pathToSSLCerts("cert.pem"), pathToSSLCerts("key.pem"), tokenToReturn)
		authorizer := createClientTokenAuthorizer(mockUAA, suppliedClientID, suppliedClientSecret)
		err := authorizer.AddAuthHeader(req, logger)
		Expect(err).NotTo(HaveOccurred())
	})

	It("caches a token", func() {
		authorizer := createClientTokenAuthorizer(mockUAA, suppliedClientID, suppliedClientSecret)
		err := authorizer.AddAuthHeader(req, logger)
		Expect(err).NotTo(HaveOccurred())

		authHeader := req.Header.Get("Authorization")
		Expect(authHeader).To(Equal(fmt.Sprintf("Bearer %s", tokenToReturn)))

		err = authorizer.AddAuthHeader(req, logger)
		Expect(err).NotTo(HaveOccurred())
		secondHeader := req.Header.Get("Authorization")
		Expect(secondHeader).To(Equal(authHeader))
		Expect(mockUAA.TokensIssued).To(Equal(1))
	})

	It("gets a new token if the previous token has expired", func() {
		authorizer := createClientTokenAuthorizer(mockUAA, suppliedClientID, suppliedClientSecret)
		err := authorizer.AddAuthHeader(req, logger)
		Expect(err).NotTo(HaveOccurred())

		authHeader := req.Header.Get("Authorization")
		Expect(authHeader).To(Equal(fmt.Sprintf("Bearer %s", tokenToReturn)))

		time.Sleep(time.Second * 2)

		err = authorizer.AddAuthHeader(req, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(mockUAA.TokensIssued).To(Equal(2))
	})

	It("fails when invalid clientID and secret are supplied", func() {
		suppliedClientID = "wrong"
		suppliedClientSecret = "wronger"

		authorizer := createClientTokenAuthorizer(mockUAA, suppliedClientID, suppliedClientSecret)
		err := authorizer.AddAuthHeader(req, logger)
		Expect(err).To(MatchError(ContainSubstring(mockuaa.UnauthorizedError)))
		Expect(err).To(MatchError(ContainSubstring(mockuaa.UnauthorizedErrorDescription)))
	})

	It("fails when receiving a malformed response from UAA", func() {
		suppliedClientID = mockuaa.MalformedResponseUser
		suppliedClientSecret = ""
		authorizer := createClientTokenAuthorizer(mockUAA, suppliedClientID, suppliedClientSecret)
		err := authorizer.AddAuthHeader(req, logger)
		Expect(err).To(MatchError(ContainSubstring("no access token in grant")))
	})

	It("fails when receiving an Internal Server Error from UAA", func() {
		suppliedClientID = mockuaa.InternalServerErrorUser
		suppliedClientSecret = ""
		authorizer := createClientTokenAuthorizer(mockUAA, suppliedClientID, suppliedClientSecret)
		err := authorizer.AddAuthHeader(req, logger)
		Expect(err).To(MatchError(ContainSubstring(mockuaa.InternalServerErrorMessage)))
	})
})

func createClientTokenAuthorizer(mockUAA *mockuaa.ClientCredentialsServer, clientID string, secret string) *authorizationheader.ClientTokenAuthHeaderBuilder {
	var certPEM []byte
	if mockUAA.TLS != nil {
		cert, err := x509.ParseCertificate(mockUAA.TLS.Certificates[0].Certificate[0])
		Expect(err).NotTo(HaveOccurred())
		certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	}
	authorizer, err := authorizationheader.NewClientTokenAuthHeaderBuilder(mockUAA.URL, clientID, secret, false, certPEM)
	Expect(err).NotTo(HaveOccurred())
	return authorizer
}
