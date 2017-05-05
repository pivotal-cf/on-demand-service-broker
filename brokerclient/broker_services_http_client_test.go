// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package brokerclient_test

import (
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokerclient"
	"github.com/pivotal-cf/on-demand-service-broker/brokerclient/broker_response"
	"github.com/pivotal-cf/on-demand-service-broker/mockbroker"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

var _ = Describe("Broker Services HTTP Client", func() {
	const (
		brokerUsername      = "username"
		brokerPassword      = "password"
		serviceInstanceGUID = "my-service-instance"
		invalidURL          = "Q#$%#$%^&&*$%^#$FGRTYW${T:WED:AWSD)E@#PE{:QS:{QLWD"
		clientTimeout       = 1 * time.Second
	)

	var (
		odb    *mockhttp.Server
		client brokerclient.BrokerServicesHTTPClient
	)

	BeforeEach(func() {
		odb = mockbroker.New()
		odb.ExpectedBasicAuth(brokerUsername, brokerPassword)
		client = brokerclient.NewBrokerServicesHTTPClient(brokerUsername, brokerPassword, odb.URL, clientTimeout)
	})

	Describe("client timeout", func() {
		AfterEach(func() {
			odb.Close()
		})

		Context("when the broker does not respond", func() {
			It("returns a timeout error", func() {
				odb.VerifyAndMock(
					mockbroker.ListInstances().DelayResponse(1 * time.Millisecond),
				)
				client = brokerclient.NewBrokerServicesHTTPClient(brokerUsername, brokerPassword, odb.URL, 1*time.Millisecond)

				_, err := client.Instances()

				Expect(err).To(HaveOccurred())
				netErr, ok := err.(net.Error)
				Expect(ok).To(BeTrue(), "is not a net.Error")
				Expect(netErr.Timeout()).To(BeTrue())
			})
		})
	})

	Describe("Instances", func() {
		AfterEach(func() {
			odb.VerifyMocks()
			odb.Close()
		})

		It("returns a list of instances", func() {
			odb.VerifyAndMock(
				mockbroker.ListInstances().RespondsOKWith(`[{"instance_id": "foo"}, {"instance_id": "bar"}]`),
			)

			instances, err := client.Instances()

			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(ConsistOf("foo", "bar"))
		})

		Context("when the url is invalid", func() {
			It("returns an error", func() {
				client := brokerclient.NewBrokerServicesHTTPClient(brokerUsername, brokerPassword, invalidURL, clientTimeout)

				_, err := client.Instances()

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				odb.Close()

				_, err := client.Instances()

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				odb.VerifyAndMock(
					mockbroker.ListInstances().RespondsOKWith(`{"not": "a list"}`),
				)

				_, err := client.Instances()

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("UpgradeInstance", func() {
		AfterEach(func() {
			odb.VerifyMocks()
			odb.Close()
		})

		It("returns an upgrade operation", func() {
			odb.VerifyAndMock(
				mockbroker.UpgradeInstance(serviceInstanceGUID).RespondsNotFoundWith(""),
			)

			upgradeOperation, err := client.UpgradeInstance(serviceInstanceGUID)

			Expect(err).NotTo(HaveOccurred())
			Expect(upgradeOperation.Type).To(Equal(broker_response.ResultNotFound))
		})

		Context("when the url is invalid", func() {
			It("returns an error", func() {
				client := brokerclient.NewBrokerServicesHTTPClient(brokerUsername, brokerPassword, invalidURL, clientTimeout)

				_, err := client.UpgradeInstance(serviceInstanceGUID)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				odb.Close()

				_, err := client.UpgradeInstance(serviceInstanceGUID)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker responds with an error", func() {
			It("returns an error", func() {
				odb.VerifyAndMock(
					mockbroker.UpgradeInstance(serviceInstanceGUID).RespondsInternalServerErrorWith("error upgrading instance"),
				)

				_, err := client.UpgradeInstance(serviceInstanceGUID)

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("LastOperation", func() {
		AfterEach(func() {
			odb.VerifyMocks()
			odb.Close()
		})

		It("returns a last operation", func() {
			operationData := broker.OperationData{
				BoshTaskID:    1,
				BoshContextID: "context-id",
				OperationType: broker.OperationTypeUpgrade,
				PlanID:        "plan-id",
			}
			expectedOperationDataJSON := `{"BoshTaskID":1,"BoshContextID":"context-id","OperationType":"upgrade","PlanID":"plan-id"}`
			odb.VerifyAndMock(
				mockbroker.LastOperation(serviceInstanceGUID, expectedOperationDataJSON).
					RespondsOKWith(`{"state":"in progress","description":"upgrade in progress"}`),
			)

			lastOperation, err := client.LastOperation(serviceInstanceGUID, operationData)

			Expect(err).NotTo(HaveOccurred())
			Expect(lastOperation).To(Equal(
				brokerapi.LastOperation{State: brokerapi.InProgress, Description: "upgrade in progress"}),
			)
		})

		Context("when the url is invalid", func() {
			It("returns an error", func() {
				client := brokerclient.NewBrokerServicesHTTPClient(brokerUsername, brokerPassword, invalidURL, clientTimeout)

				_, err := client.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				odb.Close()

				_, err := client.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the broker response is unrecognised", func() {
			It("returns an error", func() {
				expectedOperationDataJSON := `{"BoshTaskID":0,"OperationType":""}`
				odb.VerifyAndMock(
					mockbroker.LastOperation(serviceInstanceGUID, expectedOperationDataJSON).
						RespondsOKWith("invalid json"),
				)

				_, err := client.LastOperation(serviceInstanceGUID, broker.OperationData{})

				Expect(err).To(HaveOccurred())
			})
		})
	})
})
