// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package services_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/service"

	fakeclients "github.com/pivotal-cf/on-demand-service-broker/broker/services/fakes"
)

var _ = Describe("Broker Services", func() {
	const (
		serviceInstanceGUID = "my-service-instance"
		operationType       = "some-process"
	)

	var (
		brokerServices    *services.BrokerServices
		client            *fakeclients.FakeHTTPClient
		authHeaderBuilder *fakes.FakeAuthHeaderBuilder
		logger            *log.Logger
	)

	BeforeEach(func() {
		client = new(fakeclients.FakeHTTPClient)
		authHeaderBuilder = new(fakes.FakeAuthHeaderBuilder)

		loggerFactory := loggerfactory.New(os.Stdout, "broker-services-test", loggerfactory.Flags)
		logger = loggerFactory.New()
	})

	Describe("ProcessInstance", func() {
		It("returns an bosh operation", func() {
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
			planUniqueID := "unique_plan_id"
			expectedBody := fmt.Sprintf(`{"plan_id": "%s"}`, planUniqueID)
			client.DoReturns(response(http.StatusNotFound, ""), nil)

			upgradeOperation, err := brokerServices.ProcessInstance(service.Instance{
				GUID:         serviceInstanceGUID,
				PlanUniqueID: planUniqueID,
			}, operationType)

			Expect(err).NotTo(HaveOccurred())
			request := client.DoArgsForCall(0)
			Expect(request.Method).To(Equal(http.MethodPatch))
			body, err := ioutil.ReadAll(request.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(request.URL.Path).To(Equal("/mgmt/service_instances/" + serviceInstanceGUID))
			Expect(request.URL.Query()).To(Equal(url.Values{"operation_type": {operationType}}))

			Expect(upgradeOperation.Type).To(Equal(services.InstanceNotFound))
			Expect(string(body)).To(Equal(expectedBody))
		})

		It("returns an error when a new request fails to build", func() {
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "$!%#%!@#$!@%", logger)

			_, err := brokerServices.ProcessInstance(service.Instance{
				GUID:         serviceInstanceGUID,
				PlanUniqueID: "unique_plan_id",
			}, operationType)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when cannot add the authentication header", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("oops"))
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)

			_, err := brokerServices.ProcessInstance(service.Instance{
				GUID:         serviceInstanceGUID,
				PlanUniqueID: "unique_plan_id",
			}, operationType)
			Expect(err).To(HaveOccurred())
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
				client.DoReturns(nil, errors.New("connection error"))

				_, err := brokerServices.ProcessInstance(service.Instance{
					GUID:         serviceInstanceGUID,
					PlanUniqueID: "",
				}, operationType)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker responds with an error", func() {
			It("returns an error", func() {
				brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
				client.DoReturns(response(http.StatusInternalServerError, "error upgrading instance"), nil)

				_, err := brokerServices.ProcessInstance(service.Instance{
					GUID:         serviceInstanceGUID,
					PlanUniqueID: "",
				}, operationType)

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("LastOperation", func() {
		It("returns a last operation", func() {
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
			operationData := broker.OperationData{
				BoshTaskID:    1,
				BoshContextID: "context-id",
				OperationType: broker.OperationTypeUpgrade,
				PlanID:        "plan-id",
			}
			client.DoReturns(response(http.StatusOK, `{"state":"in progress","description":"upgrade in progress"}`), nil)

			lastOperation, err := brokerServices.LastOperation(serviceInstanceGUID, operationData)
			Expect(err).NotTo(HaveOccurred())

			request := client.DoArgsForCall(0)
			Expect(request.Method).To(Equal(http.MethodGet))
			Expect(err).NotTo(HaveOccurred())
			Expect(request.URL.Path).To(Equal("/v2/service_instances/" + serviceInstanceGUID + "/last_operation"))

			query, err := url.ParseQuery(request.URL.RawQuery)
			Expect(err).NotTo(HaveOccurred())
			Expect(query).To(Equal(url.Values{
				"operation": []string{`{"BoshTaskID":1,"BoshContextID":"context-id","OperationType":"upgrade","PlanID":"plan-id","PostDeployErrand":{},"PreDeleteErrand":{}}`},
			}))

			Expect(lastOperation).To(Equal(
				domain.LastOperation{State: domain.InProgress, Description: "upgrade in progress"}),
			)
		})

		It("returns an error when a new request fails to build", func() {
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "$!%#%!@#$!@%", logger)

			_, err := brokerServices.LastOperation(serviceInstanceGUID, broker.OperationData{})
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when cannot add the authentication header", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("oops"))
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)

			_, err := brokerServices.LastOperation(serviceInstanceGUID, broker.OperationData{})
			Expect(err).To(HaveOccurred())
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
				client.DoReturns(nil, errors.New("connection error"))

				_, err := brokerServices.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
				client.DoReturns(response(http.StatusOK, "invalid json"), nil)

				_, err := brokerServices.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("OrphanDeployments", func() {
		It("returns a list of orphan deployments", func() {
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
			listOfDeployments := `[{"deployment_name":"service-instance_one"},{"deployment_name":"service-instance_two"}]`
			client.DoReturns(response(http.StatusOK, listOfDeployments), nil)

			instances, err := brokerServices.OrphanDeployments()

			Expect(err).NotTo(HaveOccurred())
			request := client.DoArgsForCall(0)
			Expect(request.Method).To(Equal(http.MethodGet))
			Expect(request.URL.Path).To(Equal("/mgmt/orphan_deployments"))
			Expect(instances).To(ConsistOf(
				mgmtapi.Deployment{Name: "service-instance_one"},
				mgmtapi.Deployment{Name: "service-instance_two"},
			))
		})

		It("returns an error when a new request fails to build", func() {
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "$!%#%!@#$!@%", logger)

			_, err := brokerServices.OrphanDeployments()

			Expect(err).To(HaveOccurred())
		})

		It("returns an error when cannot add the authentication header", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("oops"))
			brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)

			_, err := brokerServices.OrphanDeployments()
			Expect(err).To(HaveOccurred())
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
				client.DoReturns(nil, errors.New("connection error"))

				_, err := brokerServices.OrphanDeployments()

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				brokerServices = services.NewBrokerServices(client, authHeaderBuilder, "http://test.test", logger)
				client.DoReturns(response(http.StatusOK, "invalid json"), nil)

				_, err := brokerServices.OrphanDeployments()

				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func response(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(strings.NewReader(body)),
	}
}
