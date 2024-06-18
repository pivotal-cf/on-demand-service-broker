// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package authorizationheader_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
)

var _ = Describe("Basic Auth Header Builder", func() {
	var req *http.Request

	BeforeEach(func() {
		var err error
		req, err = http.NewRequest("GET", "some-url-to-authorize", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	It("builds basic auth header", func() {
		authorizer := authorizationheader.NewBasicAuthHeaderBuilder("username", "password")
		err := authorizer.AddAuthHeader(req, logger)
		Expect(err).NotTo(HaveOccurred())

		authHeader := req.Header.Get("Authorization")
		Expect(authHeader).To(Equal("Basic dXNlcm5hbWU6cGFzc3dvcmQ="))
	})
})
