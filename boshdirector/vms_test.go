// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("Retriving VM Info", func() {
	const (
		deploymentName = "some-deployment"
	)

	var fakeDeployment *fakes.FakeBOSHDeployment

	BeforeEach(func() {
		vmIndex := 1
		fakeDeployment = new(fakes.FakeBOSHDeployment)
		fakeDeployment.VMInfosReturns([]director.VMInfo{
			{AgentID: "1", JobName: "some-instance-group", ID: "some-id", Index: &vmIndex, IPs: []string{"ip1", "ip2"}},
		}, nil)
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
	})

	It("returns the vms for a particular deployment", func() {
		vms, err := c.VMs(deploymentName, logger)

		By("finding the deployment")
		Expect(fakeDirector.FindDeploymentCallCount()).To(Equal(1))
		Expect(fakeDirector.FindDeploymentArgsForCall(0)).To(Equal(deploymentName))

		Expect(vms).To(HaveLen(1))
		Expect(err).NotTo(HaveOccurred())
		Expect(vms["some-instance-group"]).To(ConsistOf("ip1", "ip2"))
	})

	It("groups ips by instance group deploymentName when the output has multiple lines of the same instance group", func() {
		var vmIndex []int
		vmIndex = []int{1, 2, 3}
		fakeDeployment.VMInfosReturns([]director.VMInfo{
			{AgentID: "1", JobName: "some-instance-group", ID: "some-id", Index: &vmIndex[0], IPs: []string{"ip1"}},
			{AgentID: "1", JobName: "some-instance-group", ID: "some-id", Index: &vmIndex[1], IPs: []string{"ip2"}},
			{AgentID: "1", JobName: "other-instance-group", ID: "some-id", Index: &vmIndex[2], IPs: []string{"ip3"}},
		}, nil)

		vms, err := c.VMs(deploymentName, logger)

		Expect(err).NotTo(HaveOccurred())
		Expect(vms).To(HaveLen(2))
		Expect(vms).To(HaveKeyWithValue("some-instance-group", []string{"ip1", "ip2"}))
		Expect(vms).To(HaveKeyWithValue("other-instance-group", []string{"ip3"}))
	})

	It("errors when finding deployment fails", func() {
		fakeDirector.FindDeploymentReturns(nil, errors.New("some failure"))
		_, err := c.VMs(deploymentName, logger)
		Expect(err).To(MatchError(ContainSubstring("Could not find deployment")))
	})

	It("errors when fetching vm info fails", func() {
		fakeDeployment.VMInfosReturns(nil, errors.New("some vm info error"))
		_, err := c.VMs(deploymentName, logger)
		Expect(err).To(MatchError(ContainSubstring("Could not fetch VMs info for deployment")))
	})
})
