// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
)

var _ = Describe("verifying authentication credentials are correct", func() {
	It("doesn't produce error when the auth header builder can add a header", func() {
		director.VerifyAndMock(mockbosh.Info().RespondsOKWith("{}"))
		authHeaderBuilder := new(fakes.FakeAuthHeaderBuilder)
		c, err := boshdirector.New(director.URL, authHeaderBuilder, false, nil)
		Expect(err).NotTo(HaveOccurred())

		authErr := c.VerifyAuth(logger)

		Expect(authErr).NotTo(HaveOccurred())
	})

	It("produces error when the auth header builder cannot add a header", func() {
		authHeaderBuilder := new(fakes.FakeAuthHeaderBuilder)
		authHeaderError := errors.New("couldn't get creds!!!1 lol")
		authHeaderBuilder.AddAuthHeaderReturns(authHeaderError)
		c, err := boshdirector.New(director.URL, authHeaderBuilder, false, nil)
		Expect(err).NotTo(HaveOccurred())

		authErr := c.VerifyAuth(logger)

		Expect(authErr).To(MatchError(authHeaderError))
	})

	It("produces error when the response from the director is 401", func() {
		authHeaderBuilder := new(fakes.FakeAuthHeaderBuilder)
		director.VerifyAndMock(mockbosh.Info().RespondsUnauthorizedWith(""))
		c, err := boshdirector.New(director.URL, authHeaderBuilder, false, nil)
		Expect(err).NotTo(HaveOccurred())

		authErr := c.VerifyAuth(logger)

		Expect(authErr).To(HaveOccurred())
	})
})
