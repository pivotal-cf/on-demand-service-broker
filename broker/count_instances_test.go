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
)

var _ = Describe("counting instances of a service offering by plan", func() {
	var (
		counts   map[string]int
		countErr error

		expectedCounts      = map[string]int{"foo": 4}
		countInstancesCalls int
		logger              *log.Logger
	)

	BeforeEach(func() {
		countInstancesCalls = 0
		cfClient.CountInstancesOfServiceOfferingStub = func(id string, _ *log.Logger) (map[string]int, error) {
			countInstancesCalls++

			if countInstancesCalls == 1 {
				return map[string]int{}, nil
			}

			Expect(id).To(Equal(serviceOfferingID))
			return expectedCounts, errors.New("oh dear")
		}
	})

	JustBeforeEach(func() {
		logger = loggerFactory.NewWithRequestID()
		counts, countErr = b.CountInstancesOfPlans(logger)
	})

	It("returns the instance count", func() {
		Expect(counts).To(Equal(expectedCounts))
	})

	It("returns the error from CF client", func() {
		Expect(countErr).To(MatchError("oh dear"))
	})
})
