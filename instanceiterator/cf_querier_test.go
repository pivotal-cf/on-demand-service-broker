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

package instanceiterator_test

import (
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CF querier", func() {
	var logger *log.Logger

	BeforeEach(func() {
		loggerFactory := loggerfactory.New(gbytes.NewBuffer(), "process-all-service-instances", loggerfactory.Flags)
		logger = loggerFactory.New()
	})

	DescribeTable("CanUpgradeUsingCF",
		func(expect, maintenanceInfoPresent bool, cfClient instanceiterator.CFClient) {
			Expect(instanceiterator.CanUpgradeUsingCF(cfClient, maintenanceInfoPresent, logger)).To(Equal(expect))
		},
		Entry("MaintenanceInfo is configured and supported by CF", true, true, func() instanceiterator.CFClient {
			fake := fakes.FakeCFClient{}
			fake.CheckMinimumOSBAPIVersionReturns(true)
			return &fake
		}()),
		Entry("CF not configured", false, true, nil),
		Entry("MaintenanceInfo not configured for the adapter", false, false, &fakes.FakeCFClient{}),
		Entry("CF does not support MaintenanceInfo", false, true, &fakes.FakeCFClient{}),
	)
})
