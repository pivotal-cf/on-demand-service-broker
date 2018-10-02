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
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Orphan Deployments", func() {
	var (
		logger               *log.Logger
		orphans              []string
		orphanDeploymentsErr error
	)

	BeforeEach(func() {
		logger = loggerFactory.NewWithRequestID()
		b = createDefaultBroker()
	})

	It("returns an empty list when there are no instances or deployments", func() {
		orphans, orphanDeploymentsErr = b.OrphanDeployments(logger)

		Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
		Expect(orphans).To(BeEmpty())
	})

	It("returns an empty list when there is no orphan instances", func() {
		fakeInstanceLister.InstancesReturns([]service.Instance{{GUID: "one"}}, nil)
		boshClient.GetDeploymentsReturns([]boshdirector.Deployment{{Name: "service-instance_one"}}, nil)

		orphans, orphanDeploymentsErr = b.OrphanDeployments(logger)

		Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
		Expect(orphans).To(BeEmpty())
	})

	It("returns a list of one orphan when there are no instances but one deployment", func() {
		boshClient.GetDeploymentsReturns([]boshdirector.Deployment{{Name: "service-instance_one"}}, nil)

		orphans, orphanDeploymentsErr = b.OrphanDeployments(logger)

		Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
		Expect(orphans).To(ConsistOf("service-instance_one"))
	})

	It("ignores non-odb deployments", func() {
		deployments := []boshdirector.Deployment{{Name: "not-a-service-instance"}, {Name: "acme-deployment"}}
		boshClient.GetDeploymentsReturns(deployments, nil)

		orphans, orphanDeploymentsErr = b.OrphanDeployments(logger)

		Expect(orphanDeploymentsErr).NotTo(HaveOccurred())
		Expect(orphans).To(BeEmpty())
	})

	It("logs an error when getting the list of instances fails", func() {
		fakeInstanceLister.InstancesReturns([]service.Instance{}, errors.New("error listing instances: listing error"))

		orphans, orphanDeploymentsErr = b.OrphanDeployments(logger)

		Expect(orphanDeploymentsErr).To(HaveOccurred())
		Expect(logBuffer.String()).To(ContainSubstring("error listing instances: listing error"))
	})

	It("logs an error when getting the list of deployments fails", func() {
		boshClient.GetDeploymentsReturns([]boshdirector.Deployment{}, errors.New("error getting deployments: get deployment error"))

		orphans, orphanDeploymentsErr = b.OrphanDeployments(logger)

		Expect(orphanDeploymentsErr).To(HaveOccurred())
		Expect(logBuffer.String()).To(ContainSubstring("error getting deployments: get deployment error"))
	})
})
