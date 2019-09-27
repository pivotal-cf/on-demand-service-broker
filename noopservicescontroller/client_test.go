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

package noopservicescontroller_test

import (
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"

	"io"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"

	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Client", func() {

	var testLogger *log.Logger

	BeforeEach(func() {
		logBuffer := gbytes.NewBuffer()
		testLogger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
	})

	Describe("New", func() {
		It("should create client", func() {
			Expect(noopservicescontroller.New()).ToNot(BeNil())
		})

		It("should be CloudFoundry", func() {
			client := noopservicescontroller.New()
			var i interface{} = client
			_, ok := i.(broker.CloudFoundryClient)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("GetAPIVersion", func() {
		It("return a valid version", func() {
			client := noopservicescontroller.New()
			version, err := client.GetAPIVersion(testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(broker.MinimumCFVersion))
		})
	})

	Describe("CountInstancesOfPlan", func() {
		It("always returns 1", func() {
			client := noopservicescontroller.New()
			planCount, err := client.CountInstancesOfPlan("offeringId", "planId", testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(planCount).To(Equal(1))
		})
	})

	Describe("CountInstancesOfServiceOffering", func() {
		It("returns empty map", func() {
			client := noopservicescontroller.New()
			instanceCountByPlanID, err := client.CountInstancesOfServiceOffering("offeringId", testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(instanceCountByPlanID).ToNot(BeNil())
		})
	})

	Describe("GetInstanceState", func() {
		It("return default state", func() {
			client := noopservicescontroller.New()
			instanceState, err := client.GetInstanceState("serviceInstanceGUID", testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(instanceState).ToNot(BeNil())
		})
	})

	Describe("GetInstancesOfServiceOffering", func() {
		It("gets empty instances of service offerings", func() {
			client := noopservicescontroller.New()
			instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: "serviceInstanceGUID"}, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(instances).ToNot(BeNil())
		})
	})

})
