// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package brokercontext_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/pivotal-cf/on-demand-service-broker/brokercontext"
)

var _ = Describe("BrokerContext", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("New", func() {
		It("sets the operation, requestID, serviceName and instanceID", func() {
			operation := "create"
			requestID := "b79ebe6d-b325-45a5-8d57-7bfa2e8b18d7"
			serviceName := "the-greatest-service"
			instanceID := "the-fastest-instance-id"

			ctx = New(ctx, operation, requestID, serviceName, instanceID)

			Expect(GetOperation(ctx)).To(Equal(operation))
			Expect(GetReqID(ctx)).To(Equal(requestID))
			Expect(GetServiceName(ctx)).To(Equal(serviceName))
			Expect(GetInstanceID(ctx)).To(Equal(instanceID))
		})
	})

	Describe("Operation", func() {
		It("can be set and retrieved", func() {
			operation := "create"
			ctx = WithOperation(ctx, operation)
			Expect(GetOperation(ctx)).To(Equal(operation))
		})
	})

	Describe("Request ID", func() {
		It("can be set and retrieved", func() {
			requestId := "b79ebe6d-b325-45a5-8d57-7bfa2e8b18d7"
			ctx = WithReqID(ctx, requestId)
			Expect(GetReqID(ctx)).To(Equal(requestId))
		})
	})

	Describe("Service Name", func() {
		It("can be set and retrieved", func() {
			serviceName := "the-greatest-service"
			ctx = WithServiceName(ctx, serviceName)
			Expect(GetServiceName(ctx)).To(Equal(serviceName))
		})
	})

	Describe("Instance ID", func() {
		It("can be set and retrieved", func() {
			instanceID := "instance-id"
			ctx = WithInstanceID(ctx, instanceID)
			Expect(GetInstanceID(ctx)).To(Equal(instanceID))
		})
	})

	Describe("Bosh Task ID", func() {
		It("can be set and retrieved", func() {
			boshTaskID := 101
			ctx = WithBoshTaskID(ctx, boshTaskID)
			Expect(GetBoshTaskID(ctx)).To(Equal(boshTaskID))
		})
	})

	Context("with multiple attributes", func() {
		It("can set and retrieve all of them", func() {
			operation := "create"
			requestID := "b79ebe6d-b325-45a5-8d57-7bfa2e8b18d7"
			serviceName := "the-greatest-service"
			instanceID := "the-fastest-instance-id"
			boshTaskID := 101

			ctx = WithOperation(ctx, operation)
			ctx = WithReqID(ctx, requestID)
			ctx = WithServiceName(ctx, serviceName)
			ctx = WithInstanceID(ctx, instanceID)
			ctx = WithBoshTaskID(ctx, boshTaskID)

			Expect(GetOperation(ctx)).To(Equal(operation))
			Expect(GetReqID(ctx)).To(Equal(requestID))
			Expect(GetServiceName(ctx)).To(Equal(serviceName))
			Expect(GetInstanceID(ctx)).To(Equal(instanceID))
			Expect(GetBoshTaskID(ctx)).To(Equal(boshTaskID))
		})
	})
})
