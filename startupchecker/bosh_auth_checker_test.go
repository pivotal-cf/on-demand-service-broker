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

package startupchecker_test

import (
	. "github.com/pivotal-cf/on-demand-service-broker/startupchecker"

	"log"

	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker/fakes"
)

var _ = Describe("BOSH Auth Checker", func() {
	var (
		untestedLogger   *log.Logger
		fakeAuthVerifier *fakes.FakeAuthVerifier
	)

	BeforeEach(func() {
		fakeAuthVerifier = new(fakes.FakeAuthVerifier)
	})

	It("returns no error when auth succeeds", func() {
		fakeAuthVerifier.VerifyAuthReturns(nil)
		c := NewBOSHAuthChecker(fakeAuthVerifier, untestedLogger)
		err := c.Check()

		Expect(err).NotTo(HaveOccurred())
	})

	It("produces error when auth fails", func() {
		fakeAuthVerifier.VerifyAuthReturns(errors.New("I love errors"))
		c := NewBOSHAuthChecker(fakeAuthVerifier, untestedLogger)
		err := c.Check()

		Expect(err).To(MatchError("BOSH Director error: I love errors"))
	})
})
