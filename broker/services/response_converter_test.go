// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package services_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v10/domain"
	"github.com/pivotal-cf/brokerapi/v10/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
)

var _ = Describe("Response Converter", func() {
	var converter services.ResponseConverter

	BeforeEach(func() {
		converter = services.ResponseConverter{}
	})

	Context("last operation", func() {
		It("returns the last operation data", func() {
			expectedOperation := domain.LastOperation{
				State:       domain.InProgress,
				Description: "some-description",
			}
			response := http.Response{
				StatusCode: http.StatusOK,
				Body:       asBody(lastOperationJSON(expectedOperation)),
			}

			lastOperation, err := converter.LastOperationFrom(&response)

			Expect(err).NotTo(HaveOccurred())
			Expect(lastOperation).To(Equal(expectedOperation))
		})

		It("returns an error when the response status is not OK", func() {
			response := http.Response{
				Status:     "500 Internal Server Error",
				StatusCode: 500,
				Body:       asBody(""),
			}

			_, err := converter.LastOperationFrom(&response)

			Expect(err).To(MatchError(
				ContainSubstring("HTTP response status: 500 Internal Server Error"),
			))
		})

		It("returns an error when the response body cannot be decoded", func() {
			response := http.Response{
				StatusCode: http.StatusOK,
				Body:       asBody("{ invalid json }"),
			}

			_, err := converter.LastOperationFrom(&response)

			Expect(err).To(MatchError(ContainSubstring("invalid character")))
		})
	})

	Context("upgrade operation", func() {
		Context("when the upgrade is accepted", func() {
			It("returns the upgrade operation data", func() {
				response := http.Response{
					StatusCode: http.StatusAccepted,
					Body:       asBody(upgradeOperationJSON()),
				}

				result, err := converter.ExtractOperationFrom(&response)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Data.OperationType).To(Equal(broker.OperationTypeUpgrade))
				Expect(result.Type).To(Equal(services.OperationAccepted))
			})

			It("returns an error when the response body cannot be decoded", func() {
				response := http.Response{
					StatusCode: http.StatusAccepted,
					Body:       asBody("{ invalid json }"),
				}

				_, err := converter.ExtractOperationFrom(&response)

				Expect(err).To(MatchError(SatisfyAll(
					ContainSubstring("cannot parse upgrade response"),
					ContainSubstring("invalid character"),
				)))
			})
		})

		Context("when the cf service instance is not found", func() {
			It("returns a not found result", func() {
				response := http.Response{
					StatusCode: http.StatusNotFound,
					Body:       asBody(""),
				}

				result, err := converter.ExtractOperationFrom(&response)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Type).To(Equal(services.InstanceNotFound))
			})
		})

		Context("when bosh deployment for the service instance is gone", func() {
			It("returns an orphan service instance result", func() {
				response := http.Response{
					StatusCode: http.StatusGone,
					Body:       asBody(""),
				}

				result, err := converter.ExtractOperationFrom(&response)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Type).To(Equal(services.OrphanDeployment))
			})
		})

		Context("when the service instance has an operation in progress", func() {
			It("returns an operation in progress result", func() {
				response := http.Response{
					StatusCode: http.StatusConflict,
					Body:       asBody(""),
				}

				result, err := converter.ExtractOperationFrom(&response)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Type).To(Equal(services.OperationInProgress))
			})
		})

		Context("when the upgrade response is internal server error", func() {
			It("returns the error description", func() {
				response := http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       asBody(upgradeErrorJSON("upgrade failed")),
				}

				_, err := converter.ExtractOperationFrom(&response)

				Expect(err).To(MatchError(SatisfyAll(
					ContainSubstring("unexpected status code: 500"),
					ContainSubstring("description: upgrade failed"),
				)))
			})

			It("fails to decode the response body", func() {
				response := http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       asBody("{invalid json}"),
				}

				_, err := converter.ExtractOperationFrom(&response)

				Expect(err).To(MatchError(SatisfyAll(
					ContainSubstring("unexpected status code: 500"),
					ContainSubstring("cannot parse upgrade response: '{invalid json}'"),
				)))
			})
		})

		When("upgrade is not needed", func() {
			It("returns operation type as skipped", func() {
				response := http.Response{
					StatusCode: http.StatusNoContent,
					Body:       asBody(""),
				}

				result, err := converter.ExtractOperationFrom(&response)

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Type).To(Equal(services.OperationSkipped))
			})
		})

		Context("when the upgrade response status code is unexpected", func() {
			It("returns an error", func() {
				response := http.Response{
					StatusCode: http.StatusTeapot,
					Body:       asBody("an unexpected error occurred"),
				}

				_, err := converter.ExtractOperationFrom(&response)

				Expect(err).To(MatchError(SatisfyAll(
					ContainSubstring("unexpected status code: 418"),
					ContainSubstring("body: an unexpected error occurred"),
				)))
			})
		})
	})

	Context("orphan deployments", func() {
		It("returns orphan deployments", func() {
			response := http.Response{
				StatusCode: http.StatusOK,
				Body:       asBody(orphanDeploymentsJSON("deployment1", "deployment2")),
			}

			orphans, err := converter.OrphanDeploymentsFrom(&response)

			Expect(err).NotTo(HaveOccurred())
			Expect(orphans).To(ConsistOf(mgmtapi.Deployment{Name: "deployment1"}, mgmtapi.Deployment{Name: "deployment2"}))
		})

		It("returns an error when the response status is not OK", func() {
			response := http.Response{
				Status:     "500 Internal Server Error",
				StatusCode: 500,
				Body:       asBody(""),
			}

			_, err := converter.OrphanDeploymentsFrom(&response)

			Expect(err).To(MatchError(
				ContainSubstring("HTTP response status: 500 Internal Server Error"),
			))
		})

		It("returns an error when the response body cannot be decoded", func() {
			response := http.Response{
				StatusCode: http.StatusOK,
				Body:       asBody("{ invalid json }"),
			}

			_, err := converter.OrphanDeploymentsFrom(&response)

			Expect(err).To(MatchError(
				ContainSubstring("invalid character"),
			))
		})
	})
})

func upgradeOperationJSON() string {
	operation := broker.OperationData{
		OperationType: broker.OperationTypeUpgrade,
	}
	content, err := json.Marshal(operation)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

func upgradeErrorJSON(description string) string {
	errorResponse := apiresponses.ErrorResponse{
		Description: description,
	}
	content, err := json.Marshal(errorResponse)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

func lastOperationJSON(operation domain.LastOperation) string {
	content, err := json.Marshal(operation)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

func orphanDeploymentsJSON(deploymentNames ...string) string {
	var list []mgmtapi.Deployment
	for _, name := range deploymentNames {
		list = append(list, mgmtapi.Deployment{Name: name})
	}
	content, err := json.Marshal(list)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

func asBody(content string) io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(content))
}
