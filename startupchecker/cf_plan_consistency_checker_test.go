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

package startupchecker_test

import (
	"errors"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	. "github.com/pivotal-cf/on-demand-service-broker/startupchecker"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker/fakes"

	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CfPlanConsistencyChecker", func() {
	const existingPlanID = "existing-plan"

	var (
		client         *fakes.FakeServiceInstanceCounter
		serviceCatalog = config.ServiceOffering{
			ID: "service-id",
			Plans: []config.Plan{
				{ID: existingPlanID},
			},
		}
		noLogTesting *log.Logger
	)

	BeforeEach(func() {
		client = new(fakes.FakeServiceInstanceCounter)
	})

	It("exhibits success when no service instances deployed", func() {
		client.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{}, nil)
		c := NewCFPlanConsistencyChecker(client, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error if service catalog does not contain the existing instance's plan", func() {
		client.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
			cfServicePlan("non-existent-plan-id", "non-existent-plan"): 1,
		}, nil)

		c := NewCFPlanConsistencyChecker(client, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError(
			"plan non-existent-plan (non-existent-plan-id) was expected but is now missing. You cannot remove or change the plan_id of a plan which has existing service instances",
		))
	})

	It("returns no error when there are no pre-existing instances of configured plans and service catalog contains the existing instance's plan", func() {
		client.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
			cfServicePlan(existingPlanID, "existing-plan"): 1,
		}, nil)
		c := NewCFPlanConsistencyChecker(client, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns no error when the service catalog does not contain a plan with zero instances", func() {
		client.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
			cfServicePlan(existingPlanID, "existing-plan"):             1,
			cfServicePlan("non-existent-plan-id", "non-existent-plan"): 0,
		}, nil)
		c := NewCFPlanConsistencyChecker(client, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when instances cannot be retrieved", func() {
		client.CountInstancesOfServiceOfferingReturns(nil, errors.New("error counting instances"))

		c := NewCFPlanConsistencyChecker(client, serviceCatalog, noLogTesting)
		err := c.Check()
		Expect(err).To(HaveOccurred())
	})
})
