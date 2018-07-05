// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("verifying authentication credentials are correct", func() {
	var (
		directorClient *boshdirector.Client
	)

	BeforeEach(func() {
		var err error

		fakeDirector.InfoReturns(director.Info{
			Auth: director.UserAuthentication{
				Type: "uaa",
				Options: map[string]interface{}{
					"url": "foo.com",
				},
			},
		}, nil)

		directorClient, err = boshdirector.New("https://director.example.com", nil, fakeCertAppender, fakeDirectorFactory, fakeUAAFactory, boshAuthConfig, fakeBoshHTTPFactory.Spy, logger)
		Expect(err).NotTo(HaveOccurred())
	})

	It("doesn't produce error when the credentials are correct", func() {
		fakeDirector.IsAuthenticatedReturns(true, nil)

		authErr := directorClient.VerifyAuth(logger)

		Expect(authErr).NotTo(HaveOccurred())
	})

	It("produces an error when the credentials are incorrect", func() {
		fakeDirector.IsAuthenticatedReturns(false, nil)

		authErr := directorClient.VerifyAuth(logger)

		Expect(authErr).To(MatchError("not authenticated"))
	})

	It("produces an error when it fails to check the credentials", func() {
		errMsg := "/info endpoint unreachable"
		fakeDirector.IsAuthenticatedReturns(false, errors.New(errMsg))

		err := directorClient.VerifyAuth(logger)

		Expect(err).To(MatchError(ContainSubstring(errMsg)))
	})
})
