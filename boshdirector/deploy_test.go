// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

var _ = Describe("deploying a manifest", func() {
	const (
		taskID = 3
	)

	var (
		deploymentManifest = bosh.BoshManifest{
			Name:           "big deployment",
			Releases:       []bosh.Release{},
			Stemcells:      []bosh.Stemcell{},
			InstanceGroups: []bosh.InstanceGroup{},
		}
		deploymentManifestBytes []byte

		returnedTaskID int
		deployErr      error
	)

	BeforeEach(func() {
		var err error
		deploymentManifestBytes, err = yaml.Marshal(deploymentManifest)
		Expect(err).NotTo(HaveOccurred())
	})

	var boshContextID string

	BeforeEach(func() {
		boshContextID = ""
	})

	JustBeforeEach(func() {
		returnedTaskID, deployErr = c.Deploy(deploymentManifestBytes, boshContextID, logger)
	})

	Context("and no bosh context id", func() {
		Context("and the deploy succeeds", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Deploy().WithRawManifest(deploymentManifestBytes).WithoutContextID().RedirectsToTask(taskID),
				)
			})

			It("calls the authorization header builder", func() {
				Expect(authHeaderBuilder.BuildCallCount()).To(BeNumerically(">", 0))
			})

			It("returns the bosh task ID", func() {
				Expect(returnedTaskID).To(Equal(taskID))
			})

			It("does not return an error", func() {
				Expect(deployErr).NotTo(HaveOccurred())
			})
		})
	})

	Context("and a bosh context id", func() {
		BeforeEach(func() {
			boshContextID = "bosh-context-id"

			director.VerifyAndMock(
				mockbosh.Deploy().WithRawManifest(deploymentManifestBytes).WithContextID("bosh-context-id").RedirectsToTask(taskID),
			)
		})

		It("calls the authorization header builder", func() {
			Expect(authHeaderBuilder.BuildCallCount()).To(BeNumerically(">", 0))
		})

		It("returns the bosh task ID", func() {
			Expect(returnedTaskID).To(Equal(taskID))
		})

		It("does not return an error", func() {
			Expect(deployErr).NotTo(HaveOccurred())
		})
	})

	Context("and the authorization header cannot be built", func() {
		BeforeEach(func() {
			authHeaderBuilder.BuildReturns("", errors.New("some-error"))
		})

		It("returns an error", func() {
			Expect(deployErr).To(MatchError(ContainSubstring("some-error")))
		})
	})

	Context("and the bosh director cannot be reached", func() {
		BeforeEach(func() {
			director.Close()
		})

		It("returns an error", func() {
			Expect(deployErr).To(MatchError(ContainSubstring("error reaching bosh director")))
		})
	})

	Context("and the bosh director responds with an error", func() {
		BeforeEach(func() {
			director.VerifyAndMock(
				mockbosh.Deploy().WithoutContextID().RespondsInternalServerErrorWith("because reasons"),
			)
		})

		It("returns error", func() {
			Expect(deployErr).To(MatchError(ContainSubstring("expected status 302, was 500")))
		})
	})
})
