// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package services_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/services/fakes"
)

var _ = Describe("Broker Services", func() {
	const serviceInstanceGUID = "my-service-instance"

	var (
		brokerServices *services.BrokerServices
		client         *fakes.FakeClient
	)

	BeforeEach(func() {
		client = new(fakes.FakeClient)
		brokerServices = services.NewBrokerServices(client)
	})

	Describe("Instances", func() {
		It("returns a list of instances", func() {
			client.GetReturns(response(http.StatusOK, `[{"instance_id": "foo"}, {"instance_id": "bar"}]`), nil)

			instances, err := brokerServices.Instances()

			Expect(err).NotTo(HaveOccurred())
			actualPath, _ := client.GetArgsForCall(0)
			Expect(actualPath).To(Equal("/mgmt/service_instances"))
			Expect(instances).To(ConsistOf("foo", "bar"))
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				client.GetReturns(nil, errors.New("connection error"))

				_, err := brokerServices.Instances()

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				client.GetReturns(response(http.StatusOK, `{"not": "a list"}`), nil)

				_, err := brokerServices.Instances()

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("UpgradeInstance", func() {
		It("returns an upgrade operation", func() {
			client.PatchReturns(response(http.StatusNotFound, ""), nil)

			upgradeOperation, err := brokerServices.UpgradeInstance(serviceInstanceGUID)

			Expect(err).NotTo(HaveOccurred())
			actualPath := client.PatchArgsForCall(0)
			Expect(actualPath).To(Equal("/mgmt/service_instances/" + serviceInstanceGUID))
			Expect(upgradeOperation.Type).To(Equal(services.InstanceNotFound))
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				client.PatchReturns(nil, errors.New("connection error"))

				_, err := brokerServices.UpgradeInstance(serviceInstanceGUID)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker responds with an error", func() {
			It("returns an error", func() {
				client.PatchReturns(response(http.StatusInternalServerError, "error upgrading instance"), nil)

				_, err := brokerServices.UpgradeInstance(serviceInstanceGUID)

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("LastOperation", func() {
		It("returns a last operation", func() {
			operationData := broker.OperationData{
				BoshTaskID:    1,
				BoshContextID: "context-id",
				OperationType: broker.OperationTypeUpgrade,
				PlanID:        "plan-id",
			}
			client.GetReturns(response(http.StatusOK, `{"state":"in progress","description":"upgrade in progress"}`), nil)

			lastOperation, err := brokerServices.LastOperation(serviceInstanceGUID, operationData)

			Expect(err).NotTo(HaveOccurred())
			actualPath, actualQuery := client.GetArgsForCall(0)
			Expect(actualPath).To(Equal("/v2/service_instances/" + serviceInstanceGUID + "/last_operation"))
			Expect(actualQuery).To(Equal(map[string]string{
				"operation": `{"BoshTaskID":1,"BoshContextID":"context-id","OperationType":"upgrade","PlanID":"plan-id"}`,
			}))
			Expect(lastOperation).To(Equal(
				brokerapi.LastOperation{State: brokerapi.InProgress, Description: "upgrade in progress"}),
			)
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				client.GetReturns(nil, errors.New("connection error"))

				_, err := brokerServices.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				client.GetReturns(response(http.StatusOK, "invalid json"), nil)

				_, err := brokerServices.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("OrphanDeployments", func() {
		It("returns a list of orphan deployments", func() {
			listOfDeployments := `[{"deployment_name":"service-instance_one"},{"deployment_name":"service-instance_two"}]`
			client.GetReturns(response(http.StatusOK, listOfDeployments), nil)

			instances, err := brokerServices.OrphanDeployments()

			Expect(err).NotTo(HaveOccurred())
			actualPath, _ := client.GetArgsForCall(0)
			Expect(actualPath).To(Equal("/mgmt/orphan_deployments"))
			Expect(instances).To(ConsistOf(
				mgmtapi.Deployment{Name: "service-instance_one"},
				mgmtapi.Deployment{Name: "service-instance_two"},
			))
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				client.GetReturns(nil, errors.New("connection error"))

				_, err := brokerServices.OrphanDeployments()

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				client.GetReturns(response(http.StatusOK, "invalid json"), nil)

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
