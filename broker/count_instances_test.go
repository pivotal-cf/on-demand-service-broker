// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
)

var _ = Describe("counting instances of a service offering by plan", func() {
	It("returns the instance count when the request is successful", func() {
		expectedCounts := map[cf.ServicePlan]int{
			cfServicePlan("1234", "foo", "url", "bar"): 4,
		}

		cfClient.CountInstancesOfServiceOfferingReturns(expectedCounts, nil)
		b = createDefaultBroker()
		logger := loggerFactory.NewWithRequestID()
		counts, err := b.CountInstancesOfPlans(logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(counts).To(Equal(expectedCounts))
		instanceID, _ := cfClient.CountInstancesOfServiceOfferingArgsForCall(0)
		Expect(instanceID).To(Equal(serviceOfferingID))
	})

	It("returns an error when the request fails", func() {
		cfClient.CountInstancesOfServiceOfferingReturns(nil, errors.New("Something bad happened"))
		b = createDefaultBroker()
		logger := loggerFactory.NewWithRequestID()
		_, err := b.CountInstancesOfPlans(logger)
		Expect(err).To(MatchError("Something bad happened"))
	})
})
