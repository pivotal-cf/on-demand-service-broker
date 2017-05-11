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
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("Orphan Deployments", func() {
	var (
		logger               *log.Logger
		orphans              []string
		orphanDeploymentsErr error
	)

	BeforeEach(func() {
		logger = loggerFactory.NewWithRequestID()
	})

	JustBeforeEach(func() {
		orphans, orphanDeploymentsErr = b.OrphanDeployments(logger)
	})

	Context("when there are no instances or deployments", func() {
		It("returns an empty list", func() {
			Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
			Expect(orphans).To(BeEmpty())
		})
	})

	Context("when there is an instance with a deployment", func() {
		BeforeEach(func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{"one"}, nil)
			boshClient.GetDeploymentsReturns([]boshdirector.Deployment{{Name: "service-instance_one"}}, nil)
		})

		It("returns an empty list", func() {
			Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
			Expect(orphans).To(BeEmpty())
		})
	})

	Context("when there are no instances and one deployment", func() {
		BeforeEach(func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{}, nil)
			boshClient.GetDeploymentsReturns([]boshdirector.Deployment{{Name: "service-instance_one"}}, nil)
		})

		It("returns a list of one orphan deployment", func() {
			Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
			Expect(orphans).To(ConsistOf("service-instance_one"))
		})
	})

	Context("when there is one instance and no deployments", func() {
		BeforeEach(func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{"one"}, nil)
			boshClient.GetDeploymentsReturns([]boshdirector.Deployment{}, nil)
		})

		It("returns a list of one orphan deployment", func() {
			Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
			Expect(orphans).To(BeEmpty())
		})
	})

	Context("when there is one orphan deployment and two non-ODB deployments", func() {
		BeforeEach(func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{"one"}, nil)
			deployments := []boshdirector.Deployment{
				{Name: "service-instance_one"},
				{Name: "not-a-service-instance"},
				{Name: "acme-deployment"},
				{Name: "service-instance_two"},
			}
			boshClient.GetDeploymentsReturns(deployments, nil)
		})

		It("returns a list of one orphan deployment", func() {
			Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
			Expect(orphans).To(ConsistOf("service-instance_two"))
		})
	})

	Context("when the getting the list of instances fails", func() {
		BeforeEach(func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{}, errors.New("error listing instances: listing error"))
		})

		It("broker logs an error", func() {
			Expect(orphanDeploymentsErr).To(HaveOccurred())
			Expect(logBuffer.String()).To(ContainSubstring("error listing instances: listing error"))
		})
	})

	Context("when the getting the list of deployments fails", func() {
		BeforeEach(func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{"one"}, nil)
			boshClient.GetDeploymentsReturns([]boshdirector.Deployment{}, errors.New("error getting deployments: get deployment error"))
		})

		It("broker logs an error", func() {
			Expect(orphanDeploymentsErr).To(HaveOccurred())
			Expect(logBuffer.String()).To(ContainSubstring("error getting deployments: get deployment error"))
		})
	})
})
