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
	"fmt"

	. "github.com/pivotal-cf/on-demand-service-broker/startupchecker"

	"errors"

	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker/fakes"
)

var _ = Describe("CFAPIVersionChecker", func() {
	const (
		minimumCFVersion = "2.57.0"
		oldCFVersion     = "2.56.0"
		invalidCFVersion = "1.invalid.0"
	)

	var (
		client       *fakes.FakeCFAPIVersionGetter
		noLogTesting *log.Logger
	)

	BeforeEach(func() {
		client = new(fakes.FakeCFAPIVersionGetter)
	})

	It("exhibits success when CF API is current", func() {
		client.GetAPIVersionReturns(minimumCFVersion, nil)
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("exhibits success when CF API is the next major version", func() {
		client.GetAPIVersionReturns("3.0.0", nil)
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("produces error when CF API is out of date", func() {
		client.GetAPIVersionReturns(oldCFVersion, nil)
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError(fmt.Sprintf("CF API error: ODB requires minimum Cloud Foundry API version '%s', got '%s'.", minimumCFVersion, oldCFVersion)))
	})

	It("produces error when minimum CF version is invalid", func() {
		c := NewCFAPIVersionChecker(client, "foo", noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError("Could not parse configured minimum Cloud Foundry API version. Expected a semver, got: foo"))
	})

	It("produces error when CF API responds with error", func() {
		cfAPIFailureMessage := "Failed to contact CF API"
		client.GetAPIVersionReturns("", errors.New(cfAPIFailureMessage))
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError(fmt.Sprintf("CF API error: %s. ODB requires minimum Cloud Foundry API version: %s", cfAPIFailureMessage, minimumCFVersion)))
	})

	It("produces error if the CF API version cannot be parsed", func() {
		client.GetAPIVersionReturns(invalidCFVersion, nil)
		c := NewCFAPIVersionChecker(client, minimumCFVersion, noLogTesting)
		err := c.Check()
		Expect(err).To(MatchError("Cloud Foundry API version couldn't be parsed. Expected a semver, got: 1.invalid.0."))
	})
})

func cfServicePlan(uniqueID, name string) cf.ServicePlan {
	return cf.ServicePlan{
		ServicePlanEntity: cf.ServicePlanEntity{
			UniqueID: uniqueID,
			Name:     name,
		},
	}
}
