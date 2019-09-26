// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("getting deployment", func() {
	var (
		deploymentName         = "some-deployment"
		manifest               []byte
		deploymentFound        bool
		rawManifest            = []byte("a-raw-manifest")
		manifestFetchErr       error
		fakeDeployment         *fakes.FakeBOSHDeployment
		fakeDeploymentResponse director.DeploymentResp
	)

	BeforeEach(func() {
		fakeDeployment = new(fakes.FakeBOSHDeployment)
		fakeDeployment.ManifestReturns(string(rawManifest), nil)

		fakeDeploymentResponse = director.DeploymentResp{
			Name: deploymentName,
		}

		fakeDirector.ListDeploymentsReturns([]director.DeploymentResp{fakeDeploymentResponse}, nil)
	})

	It("returns the manifest when the deployment exists", func() {
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
		fakeDeployment.ManifestReturns(string(rawManifest), nil)

		manifest, deploymentFound, manifestFetchErr = c.GetDeployment(deploymentName, logger)

		By("finding the deployment")
		Expect(fakeDirector.ListDeploymentsCallCount()).To(Equal(1))

		By("acquiring the manifest")
		Expect(fakeDeployment.ManifestCallCount()).To(Equal(1))

		By("returning the manifest")
		Expect(deploymentFound).To(BeTrue())
		Expect(manifest).To(Equal(rawManifest))
		Expect(manifestFetchErr).NotTo(HaveOccurred())
	})

	It("returns a nil manifest and false but without an error when the deployment is not found", func() {
		_, deploymentFound, manifestFetchErr = c.GetDeployment("some-other-name", logger)

		Expect(deploymentFound).To(BeFalse())
		Expect(manifestFetchErr).NotTo(HaveOccurred())
	})

	It("returns an error when the client cannot get the list of the deployments", func() {
		fakeDirector.ListDeploymentsReturns(nil, errors.New("oops"))
		_, deploymentFound, manifestFetchErr = c.GetDeployment(deploymentName, logger)

		Expect(deploymentFound).To(BeFalse())
		Expect(manifestFetchErr).To(MatchError(ContainSubstring("Cannot get the list of deployments")))
	})

	It("returns an error when the deployment object cannot be created", func() {
		fakeDirector.FindDeploymentReturns(nil, errors.New("cannot find deployment"))

		_, _, manifestFetchErr = c.GetDeployment(deploymentName, logger)
		Expect(manifestFetchErr).To(MatchError(ContainSubstring(`Cannot create deployment object for deployment "some-deployment"`)))
	})

	It("returns an error when the manifest cannot be downloaded", func() {
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
		fakeDeployment.ManifestReturns("", errors.New("fake manifest error"))

		_, _, manifestFetchErr = c.GetDeployment(deploymentName, logger)
		Expect(manifestFetchErr).To(MatchError(ContainSubstring(`Cannot obtain manifest for deployment "some-deployment"`)))
	})
})
