// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package network_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/network"
	"github.com/pivotal-cf/on-demand-service-broker/network/fakes"
	"net/url"
)

var _ = Describe("Basic Auth HTTP Client", func() {
	const (
		username    = "username"
		password    = "password"
		baseURL     = "http://example.com:8080"
		invalidURL  = "http://a b.com"
		invalidPath = "%"
	)

	Describe("GET", func() {
		It("sets the URL", func() {
			doer := new(fakes.FakeDoer)
			client := network.NewBasicAuthHTTPClient(doer, username, password, baseURL)

			_, err := client.Get("path/to/resource", nil)

			Expect(err).NotTo(HaveOccurred())
			actualRequest := doer.DoArgsForCall(0)
			Expect(actualRequest.Method).To(Equal("GET"))
			Expect(actualRequest.URL.String()).To(Equal("http://example.com:8080/path/to/resource"))
		})

		It("normalises trailing base URL slashes and leading path slashes", func() {
			doer := new(fakes.FakeDoer)
			client := network.NewBasicAuthHTTPClient(doer, username, password, "http://example.com:8080/")

			_, err := client.Get("/path/to/resource", nil)

			Expect(err).NotTo(HaveOccurred())
			actualRequest := doer.DoArgsForCall(0)
			Expect(actualRequest.Method).To(Equal("GET"))
			Expect(actualRequest.URL.String()).To(Equal("http://example.com:8080/path/to/resource"))
		})

		It("sets basic auth", func() {
			doer := new(fakes.FakeDoer)
			client := network.NewBasicAuthHTTPClient(doer, username, password, baseURL)

			_, err := client.Get("path/to/resource", nil)

			Expect(err).NotTo(HaveOccurred())
			actualUsername, actualPassword, ok := doer.DoArgsForCall(0).BasicAuth()
			Expect(ok).To(BeTrue())
			Expect(actualUsername).To(Equal(username))
			Expect(actualPassword).To(Equal(password))
		})

		It("sets query params", func() {
			doer := new(fakes.FakeDoer)
			client := network.NewBasicAuthHTTPClient(doer, username, password, baseURL)
			params := map[string]string{
				"param1": "value1",
				"param2": "value2",
			}

			_, err := client.Get("path/to/resource", params)

			Expect(err).NotTo(HaveOccurred())
			Expect(doer.DoArgsForCall(0).URL.Query()).To(Equal(url.Values{
				"param1": {"value1"},
				"param2": {"value2"},
			}))
		})

		It("errors when the base URL is invalid", func() {
			client := network.NewBasicAuthHTTPClient(nil, username, password, invalidURL)

			_, err := client.Get("path/to/resource", nil)

			Expect(err).To(HaveOccurred())
		})

		It("errors when the path is invalid", func() {
			client := network.NewBasicAuthHTTPClient(nil, username, password, baseURL)

			_, err := client.Get(invalidPath, nil)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("PATCH", func() {
		It("sets the URL", func() {
			doer := new(fakes.FakeDoer)
			client := network.NewBasicAuthHTTPClient(doer, username, password, baseURL)

			_, err := client.Patch("path/to/resource")

			Expect(err).NotTo(HaveOccurred())
			actualRequest := doer.DoArgsForCall(0)
			Expect(actualRequest.Method).To(Equal("PATCH"))
			Expect(actualRequest.URL.String()).To(Equal("http://example.com:8080/path/to/resource"))
		})

		It("sets basic auth", func() {
			doer := new(fakes.FakeDoer)
			client := network.NewBasicAuthHTTPClient(doer, username, password, baseURL)

			_, err := client.Patch("path/to/resource")

			Expect(err).NotTo(HaveOccurred())
			actualUsername, actualPassword, ok := doer.DoArgsForCall(0).BasicAuth()
			Expect(ok).To(BeTrue())
			Expect(actualUsername).To(Equal(username))
			Expect(actualPassword).To(Equal(password))
		})

		It("errors when the base URL is invalid", func() {
			client := network.NewBasicAuthHTTPClient(nil, username, password, invalidURL)

			_, err := client.Patch("path/to/resource")

			Expect(err).To(HaveOccurred())
		})

		It("errors when the path is invalid", func() {
			client := network.NewBasicAuthHTTPClient(nil, username, password, baseURL)

			_, err := client.Patch(invalidPath)

			Expect(err).To(HaveOccurred())
		})
	})
})
