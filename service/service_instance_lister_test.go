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

package service_test

import (
	"errors"

	"github.com/pivotal-cf/on-demand-service-broker/service"

	"io/ioutil"
	"net/http"
	"strings"

	"fmt"

	"net/url"

	"crypto/x509"

	"log"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	fakes2 "github.com/pivotal-cf/on-demand-service-broker/authorizationheader/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/service/fakes"
)

var _ = Describe("ServiceInstanceLister", func() {
	var (
		client            *fakes.FakeDoer
		authHeaderBuilder *fakes2.FakeAuthHeaderBuilder
		logger            *log.Logger
	)

	BeforeEach(func() {
		client = new(fakes.FakeDoer)
		authHeaderBuilder = new(fakes2.FakeAuthHeaderBuilder)
		loggerFactory := loggerfactory.New(os.Stdout, "service-instance-lister-test", loggerfactory.Flags)
		logger = loggerFactory.New()
	})

	It("lists service instances", func() {
		client.DoReturns(response(http.StatusOK, `[{"service_instance_id": "foo", "plan_id": "plan"}, {"service_instance_id": "bar", "plan_id": "another-plan"}]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)

		instances, err := serviceInstanceLister.FilteredInstances(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(len(instances)).To(Equal(2))
		Expect(instances[0]).To(Equal(service.Instance{GUID: "foo", PlanUniqueID: "plan"}))
	})

	It("returns an error when the request fails", func() {
		client.DoReturns(nil, errors.New("connection error"))
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)
		_, err := serviceInstanceLister.FilteredInstances(nil)
		Expect(err).To(MatchError("connection error"))
	})

	It("returns an error when the broker response is unrecognised", func() {
		client.DoReturns(response(http.StatusOK, `{"not": "a list"}`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)
		_, err := serviceInstanceLister.FilteredInstances(nil)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when the HTTP status is not OK", func() {
		client.DoReturns(response(http.StatusInternalServerError, `{"description": "oops", "another-field": "ignored"}`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)
		_, err := serviceInstanceLister.FilteredInstances(nil)
		Expect(err).To(MatchError(fmt.Sprintf(
			"HTTP response status: %d %s. %s",
			http.StatusInternalServerError,
			http.StatusText(http.StatusInternalServerError),
			"oops",
		)))
	})

	It("returns a service instance API error when the HTTP status is not OK and service API is configured", func() {
		client.DoReturns(response(http.StatusInternalServerError, `not json description, so not shown in error`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", true, logger)
		_, err := serviceInstanceLister.FilteredInstances(nil)
		Expect(err).To(MatchError(fmt.Sprintf(
			"error communicating with service_instances_api (%s): HTTP response status: %d %s. %s",
			"http://example.org/some-path",
			http.StatusInternalServerError,
			http.StatusText(http.StatusInternalServerError),
			"",
		)))
	})

	It("returns SSL validation error when service instance API request fails due to unknown authority", func() {
		expectedURL := "https://example.org/service-instances"
		expectedError := &url.Error{
			URL: expectedURL,
			Err: x509.UnknownAuthorityError{},
		}
		client.DoReturns(nil, expectedError)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, expectedURL, true, logger)
		_, err := serviceInstanceLister.FilteredInstances(nil)
		Expect(err).To(MatchError(fmt.Sprintf(
			"SSL validation error for `service_instances_api.url`: %s. Please configure a `service_instances_api.root_ca_cert` and use a valid SSL certificate",
			expectedURL,
		)))
	})

	It("returns the expected error when service instance API request fails due to generic certificate error", func() {
		expectedURL := "https://example.org/service-instances"
		expectedError := &url.Error{
			URL: expectedURL,
			Err: x509.CertificateInvalidError{},
		}
		client.DoReturns(nil, expectedError)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, expectedURL, true, logger)
		_, err := serviceInstanceLister.FilteredInstances(nil)
		Expect(err).To(MatchError(Equal(fmt.Sprintf(
			"error communicating with service_instances_api (%s): %s",
			expectedURL,
			expectedError.Error(),
		))))
	})

	It("returns the expected error when service instance API request fails due to a url error with no Err", func() {
		expectedURL := "https://example.org/service-instances"
		expectedError := &url.Error{
			URL: expectedURL,
		}
		client.DoReturns(nil, expectedError)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, expectedURL, true, logger)
		_, err := serviceInstanceLister.FilteredInstances(nil)
		Expect(err).To(Equal(expectedError))
	})

	It("passes expected request to authHeaderBuilder", func() {
		expectedURL := "https://example.org/service-instances"
		client.DoReturns(response(http.StatusOK, `[]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, expectedURL, false, logger)
		serviceInstanceLister.FilteredInstances(nil)
		requestForAuth, _ := authHeaderBuilder.AddAuthHeaderArgsForCall(0)
		expectedRequest, err := http.NewRequest(
			http.MethodGet,
			expectedURL,
			nil,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(requestForAuth).To(Equal(expectedRequest))
	})

	It("passes expected request to httpClient", func() {
		expectedURL := "https://example.org/service-instances"
		client.DoReturns(response(http.StatusOK, `[]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, expectedURL, false, logger)
		serviceInstanceLister.FilteredInstances(nil)
		requestForClient := client.DoArgsForCall(0)
		expectedRequest, err := http.NewRequest(
			http.MethodGet,
			expectedURL,
			nil,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(requestForClient).To(Equal(expectedRequest))
	})

	It("passes logger to authHeaderBuilder", func() {
		expectedURL := "https://example.org/service-instances"
		client.DoReturns(response(http.StatusOK, `[]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, expectedURL, false, logger)
		serviceInstanceLister.FilteredInstances(nil)
		_, actualLogger := authHeaderBuilder.AddAuthHeaderArgsForCall(0)
		Expect(actualLogger).To(BeIdenticalTo(logger))
	})

	It("refreshes an instance", func() {
		client.DoReturnsOnCall(0, response(http.StatusOK, `[{"service_instance_id": "foo", "plan_id": "plan"}, {"service_instance_id": "bar", "plan_id": "another-plan"}]`), nil)
		client.DoReturnsOnCall(1, response(http.StatusOK, `[{"service_instance_id": "foo", "plan_id": "plan2"}, {"service_instance_id": "bar", "plan_id": "another-plan"}]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)

		instance, err := serviceInstanceLister.LatestInstanceInfo(service.Instance{GUID: "foo"})
		Expect(err).NotTo(HaveOccurred())
		Expect(instance).To(Equal(service.Instance{GUID: "foo", PlanUniqueID: "plan"}))

		instance, err = serviceInstanceLister.LatestInstanceInfo(service.Instance{GUID: "foo"})
		Expect(err).NotTo(HaveOccurred())
		Expect(instance).To(Equal(service.Instance{GUID: "foo", PlanUniqueID: "plan2"}))
	})

	It("returns a instance not found error when instance is not found", func() {
		client.DoReturns(response(http.StatusOK, `[{"service_instance_id": "foo", "plan_id": "plan"}, {"service_instance_id": "bar", "plan_id": "another-plan"}]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)
		_, err := serviceInstanceLister.LatestInstanceInfo(service.Instance{GUID: "qux"})
		Expect(err).To(Equal(service.InstanceNotFound))
	})

	It("returns an error when pulling the list of instances fail", func() {
		client.DoReturns(response(http.StatusBadRequest, `[]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)
		_, err := serviceInstanceLister.LatestInstanceInfo(service.Instance{GUID: "foo"})
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("Bad Request")))
	})

	It("returns an error when pulling the list of instances fail", func() {
		client.DoReturns(response(http.StatusServiceUnavailable, `[]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)
		_, err := serviceInstanceLister.LatestInstanceInfo(service.Instance{GUID: "foo"})
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("503 Service Unavailable")))
	})

	Describe("requesting filtered instances", func() {
		var (
			params map[string]string
		)

		BeforeEach(func() {
			params = map[string]string{
				"org":   "my-org",
				"space": "my-space",
			}
		})

		It("uses filter params", func() {
			client.DoReturns(response(http.StatusOK, `[{"service_instance_id": "foo", "plan_id": "plan"}, {"service_instance_id": "bar", "plan_id": "another-plan"}]`), nil)
			serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "https://odb.example.com", false, logger)
			filteredInstances, err := serviceInstanceLister.FilteredInstances(params)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.DoCallCount()).To(Equal(1))
			req := client.DoArgsForCall(0)
			Expect(req.URL.RawQuery).To(Equal("org=my-org&space=my-space"))
			Expect(req.URL.Host).To(Equal("odb.example.com"))

			Expect(authHeaderBuilder.AddAuthHeaderCallCount()).To(Equal(1))
			authReq, authLogger := authHeaderBuilder.AddAuthHeaderArgsForCall(0)
			Expect(authReq).To(Equal(req))
			Expect(authLogger).To(Equal(logger))

			Expect(filteredInstances).To(Equal([]service.Instance{
				service.Instance{
					GUID:         "foo",
					PlanUniqueID: "plan",
				},
				service.Instance{
					GUID:         "bar",
					PlanUniqueID: "another-plan",
				},
			}))
		})

		It("does not filter params", func() {
			client.DoReturns(response(http.StatusOK, `[{"service_instance_id": "foo", "plan_id": "plan"}, {"service_instance_id": "bar", "plan_id": "another-plan"}]`), nil)
			serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "https://odb.example.com", false, logger)
			_, err := serviceInstanceLister.FilteredInstances(nil)
			Expect(err).NotTo(HaveOccurred())
			req := client.DoArgsForCall(0)
			Expect(req.URL.RawQuery).To(Equal(""))
			Expect(req.URL.Host).To(Equal("odb.example.com"))

		})

		It("fails if cannot retrieve the auth header", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("oops"))
			serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "https://odb.example.com", false, logger)
			_, err := serviceInstanceLister.FilteredInstances(params)
			Expect(err).To(MatchError(ContainSubstring("oops")))
		})

		It("returns an error when pulling the list of instances fail", func() {
			client.DoReturns(response(http.StatusBadRequest, `[]`), nil)
			serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, "", false, logger)
			_, err := serviceInstanceLister.FilteredInstances(params)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("Bad Request")))
		})

		It("returns the expected error when service instance API request fails due to a url error with no Err", func() {
			expectedURL := "https://example.org/service-instances"
			expectedError := &url.Error{
				URL: expectedURL,
			}
			client.DoReturns(nil, expectedError)
			serviceInstanceLister := service.NewInstanceLister(client, authHeaderBuilder, expectedURL, true, logger)
			_, err := serviceInstanceLister.FilteredInstances(params)
			Expect(err).To(Equal(expectedError))
		})
	})
})

func response(statusCode int, body string) *http.Response {
	parsedUrl, err := url.Parse("http://example.org/some-path")
	Expect(err).NotTo(HaveOccurred())
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Body:       ioutil.NopCloser(strings.NewReader(body)),
		Request: &http.Request{
			URL: parsedUrl,
		},
	}
}
