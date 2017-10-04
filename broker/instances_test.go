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

var _ = Describe("Instances", func() {
	Describe("listing all instances", func() {
		var logger *log.Logger

		BeforeEach(func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{"red", "green", "blue"}, nil)
			logger = loggerFactory.NewWithRequestID()
		})

		It("returns a list of instance IDs", func() {
			b = createDefaultBroker()
			Expect(b.Instances(logger)).To(ConsistOf("red", "green", "blue"))
		})

		Context("when the list of instances cannot be retrieved", func() {
			BeforeEach(func() {
				cfClient.GetInstancesOfServiceOfferingReturns(nil, errors.New("an error occurred"))
			})

			It("returns an error", func() {
				b = createDefaultBroker()
				_, err := b.Instances(logger)
				Expect(err).To(MatchError(ContainSubstring("an error occurred")))
			})
		})
	})
})
