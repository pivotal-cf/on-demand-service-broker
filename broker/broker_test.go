// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
)

var _ = Describe("Initializing the broker", func() {
	It("returns a broker with no error when there are no startupChecks", func() {
		generatedBroker, err := createBroker([]broker.StartupChecker{})
		Expect(err).NotTo(HaveOccurred())

		Expect(generatedBroker).NotTo(BeNil())
	})

	It("returns with no error when all startupChecks return successfully", func() {
		firstFakeStartupChecker := &fakes.FakeStartupChecker{}
		firstFakeStartupChecker.CheckReturns(nil)
		secondFakeStartupChecker := &fakes.FakeStartupChecker{}
		secondFakeStartupChecker.CheckReturns(nil)
		_, err := createBroker([]broker.StartupChecker{firstFakeStartupChecker, secondFakeStartupChecker})
		Expect(err).NotTo(HaveOccurred())
	})

	It("fails with a sensible error message when there is a single failing checker", func() {
		fakeStartupChecker := &fakes.FakeStartupChecker{}
		fakeStartupChecker.CheckReturns(errors.New("a fake error"))
		_, err := createBroker([]broker.StartupChecker{fakeStartupChecker})
		Expect(err).To(MatchError(ContainSubstring("a fake error")))
	})

	It("fails with both errors when there are two failing checker", func() {
		firstFakeStartupChecker := &fakes.FakeStartupChecker{}
		firstFakeStartupChecker.CheckReturns(errors.New("the first fake error"))
		secondFakeStartupChecker := &fakes.FakeStartupChecker{}
		secondFakeStartupChecker.CheckReturns(errors.New("the second fake error"))
		_, err := createBroker([]broker.StartupChecker{firstFakeStartupChecker, secondFakeStartupChecker})
		Expect(err).To(MatchError("The following broker startup checks failed: the first fake error; the second fake error"))
	})
})
