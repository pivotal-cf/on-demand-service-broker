// Copyright (C) 2018-Present Pivotal Software, Inc. All rights reserved.
//
// This program and the accompanying materials are made available under the
// terms of the under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
//
// You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package boshdirector_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("Variables", func() {
	var fakeDeployment *fakes.FakeBOSHDeployment

	BeforeEach(func() {
		fakeDeployment = new(fakes.FakeBOSHDeployment)
	})

	It("returns a list of variables for a given deployment", func() {
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
		fakeDeployment.VariablesReturns([]director.VariableResult{
			{Name: "gordon", ID: "123"},
			{Name: "bennett", ID: "456"},
		}, nil)

		variables, err := c.Variables("some-deployment", logger)
		Expect(err).NotTo(HaveOccurred())

		Expect(variables).To(Equal([]boshdirector.Variable{
			{Path: "gordon", ID: "123"},
			{Path: "bennett", ID: "456"},
		}))
	})

	Describe("error handling", func() {
		It("fails when the director can't be built", func() {
			fakeDirectorFactory.NewReturns(nil, errors.New("boom"))
			_, err := c.Variables("some-deployment", logger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to build director: boom"))
		})

		It("fails when the deployment can't be found", func() {
			fakeDirector.FindDeploymentReturns(fakeDeployment, errors.New("boom"))
			_, err := c.Variables("some-deployment", logger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("can't find deployment with name 'some-deployment': boom"))
		})

		It("fails when the variables can't be retrieved", func() {
			fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
			fakeDeployment.VariablesReturns(nil, errors.New("kaboom"))
			_, err := c.Variables("some-deployment", logger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("can't retrieve variables for deployment 'some-deployment': kaboom"))
		})
	})
})
