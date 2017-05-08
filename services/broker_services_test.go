// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package services_test

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/services"
)

var _ = Describe("Broker Services HTTP Client", func() {
	const (
		brokerURL           = "http://example.com:8080"
		brokerUsername      = "username"
		brokerPassword      = "password"
		serviceInstanceGUID = "my-service-instance"
		invalidURL          = "Q#$%#$%^&&*$%^#$FGRTYW${T:WED:AWSD)E@#PE{:QS:{QLWD"
	)

	var (
		client     services.BrokerServices
		httpClient *fakeClient
	)

	BeforeEach(func() {
		httpClient = new(fakeClient)
		httpClient.ExpectsBasicAuth(brokerUsername, brokerPassword)
		client = services.NewBrokerServices(brokerUsername, brokerPassword, brokerURL, httpClient)
	})

	Describe("Instances", func() {
		BeforeEach(func() {
			httpClient.ExpectsRequest("GET", brokerURL, "/mgmt/service_instances")
		})

		It("returns a list of instances", func() {
			httpClient.DoReturnsResponse(http.StatusOK, `[{"instance_id": "foo"}, {"instance_id": "bar"}]`)

			instances, err := client.Instances()

			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(ConsistOf("foo", "bar"))
		})

		Context("when the url is invalid", func() {
			It("returns an error", func() {
				client := services.NewBrokerServices(brokerUsername, brokerPassword, invalidURL, httpClient)

				_, err := client.Instances()

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				httpClient.DoReturnsError(errors.New("connection error"))

				_, err := client.Instances()

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				httpClient.DoReturnsResponse(http.StatusOK, `{"not": "a list"}`)

				_, err := client.Instances()

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("UpgradeInstance", func() {
		BeforeEach(func() {
			httpClient.ExpectsRequest("PATCH", brokerURL, "/mgmt/service_instances/"+serviceInstanceGUID)
		})

		It("returns an upgrade operation", func() {
			httpClient.DoReturnsResponse(http.StatusNotFound, "")

			upgradeOperation, err := client.UpgradeInstance(serviceInstanceGUID)

			Expect(err).NotTo(HaveOccurred())
			Expect(upgradeOperation.Type).To(Equal(services.InstanceNotFound))
		})

		Context("when the url is invalid", func() {
			It("returns an error", func() {
				client := services.NewBrokerServices(brokerUsername, brokerPassword, invalidURL, httpClient)

				_, err := client.UpgradeInstance(serviceInstanceGUID)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				httpClient.DoReturnsError(errors.New("connection error"))

				_, err := client.UpgradeInstance(serviceInstanceGUID)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker responds with an error", func() {
			It("returns an error", func() {
				httpClient.DoReturnsResponse(http.StatusInternalServerError, "error upgrading instance")

				_, err := client.UpgradeInstance(serviceInstanceGUID)

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("LastOperation", func() {
		BeforeEach(func() {
			httpClient.ExpectsRequest("GET", brokerURL, "/v2/service_instances/"+serviceInstanceGUID+"/last_operation")
		})

		It("returns a last operation", func() {
			operationData := broker.OperationData{
				BoshTaskID:    1,
				BoshContextID: "context-id",
				OperationType: broker.OperationTypeUpgrade,
				PlanID:        "plan-id",
			}
			expectedOperationDataJSON := `{"BoshTaskID":1,"BoshContextID":"context-id","OperationType":"upgrade","PlanID":"plan-id"}`
			httpClient.DoReturnsResponse(http.StatusOK, `{"state":"in progress","description":"upgrade in progress"}`)
			httpClient.ExpectsRequestWithQueryParam("GET", brokerURL, "/v2/service_instances/"+serviceInstanceGUID+"/last_operation", "operation", expectedOperationDataJSON)

			lastOperation, err := client.LastOperation(serviceInstanceGUID, operationData)

			Expect(err).NotTo(HaveOccurred())
			Expect(lastOperation).To(Equal(
				brokerapi.LastOperation{State: brokerapi.InProgress, Description: "upgrade in progress"}),
			)
		})

		Context("when the url is invalid", func() {
			It("returns an error", func() {
				client := services.NewBrokerServices(brokerUsername, brokerPassword, invalidURL, httpClient)

				_, err := client.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				httpClient.DoReturnsError(errors.New("connection error"))

				_, err := client.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				httpClient.DoReturnsResponse(http.StatusOK, "invalid json")

				_, err := client.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})
	})
})

type fakeClient struct {
	response                    *http.Response
	err                         error
	expectedAuthorizationHeader string
	expectedMethod              string
	expectedURL                 string
	expectedQueryParam          string
	expectedQueryValue          string
}

func (f *fakeClient) ExpectsBasicAuth(username, password string) {
	rawBasicAuth := username + ":" + password
	f.expectedAuthorizationHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(rawBasicAuth))
}

func (f *fakeClient) ExpectsRequest(method, baseURL, path string) {
	f.expectedMethod = method
	f.expectedURL = baseURL + path
}

func (f *fakeClient) ExpectsRequestWithQueryParam(method, baseURL, path, param, value string) {
	f.ExpectsRequest(method, baseURL, path)
	f.expectedQueryParam = param
	f.expectedQueryValue = value
}

func (f *fakeClient) Do(request *http.Request) (*http.Response, error) {
	Expect(request.Header.Get("Authorization")).To(Equal(f.expectedAuthorizationHeader))
	Expect(request.Method).To(Equal(f.expectedMethod))
	Expect(strings.HasPrefix(request.URL.String(), f.expectedURL)).To(BeTrue())
	if f.expectedQueryParam != "" {
		Expect(request.URL.Query().Get(f.expectedQueryParam)).To(Equal(f.expectedQueryValue))
	}
	return f.response, f.err
}

func (f *fakeClient) DoReturnsResponse(statusCode int, body string) {
	f.response = &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(strings.NewReader(body)),
	}
}

func (f *fakeClient) DoReturnsError(err error) {
	f.err = err
}
