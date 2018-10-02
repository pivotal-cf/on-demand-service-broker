// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"errors"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Instances", func() {

	var logger *log.Logger
	BeforeEach(func() {
		logger = loggerFactory.NewWithRequestID()

	})

	Describe("listing all instances", func() {
		BeforeEach(func() {
			b = createDefaultBroker()
		})

		It("returns a list of instance IDs", func() {
			fakeInstanceLister.InstancesReturns([]service.Instance{
				{GUID: "red", PlanUniqueID: "colour-plan"},
				{GUID: "green", PlanUniqueID: "colour-plan"},
				{GUID: "blue", PlanUniqueID: "colour-plan"},
			}, nil)

			Expect(b.Instances(logger)).To(ConsistOf(
				service.Instance{GUID: "red", PlanUniqueID: "colour-plan"},
				service.Instance{GUID: "green", PlanUniqueID: "colour-plan"},
				service.Instance{GUID: "blue", PlanUniqueID: "colour-plan"},
			))
		})

		It("returns an error when the list of instances cannot be retrieved", func() {
			fakeInstanceLister.InstancesReturns(nil, errors.New("an error occurred"))

			_, err := b.Instances(logger)
			Expect(err).To(MatchError(ContainSubstring("an error occurred")))
		})
	})

	Describe("listing all instances by org and space", func() {
		var logger *log.Logger

		BeforeEach(func() {
			cfClient.GetInstancesOfServiceOfferingByOrgSpaceReturns([]service.Instance{
				{GUID: "red", PlanUniqueID: "colour-plan"},
				{GUID: "green", PlanUniqueID: "colour-plan"},
				{GUID: "blue", PlanUniqueID: "colour-plan"},
			}, nil)
			logger = loggerFactory.NewWithRequestID()
		})

		It("returns a list of instance IDs", func() {
			b = createDefaultBroker()
			Expect(b.FilteredInstances("cf-org", "cf-space", logger)).To(ConsistOf(
				service.Instance{GUID: "red", PlanUniqueID: "colour-plan"},
				service.Instance{GUID: "green", PlanUniqueID: "colour-plan"},
				service.Instance{GUID: "blue", PlanUniqueID: "colour-plan"},
			))
		})

		Context("when the list of instances cannot be retrieved", func() {
			BeforeEach(func() {
				cfClient.GetInstancesOfServiceOfferingByOrgSpaceReturns(nil, errors.New("an error occurred"))
			})

			It("returns an error", func() {
				b = createDefaultBroker()
				_, err := b.FilteredInstances("cf-org", "cf-space", logger)
				Expect(err).To(MatchError(ContainSubstring("an error occurred")))
			})
		})
	})
})
