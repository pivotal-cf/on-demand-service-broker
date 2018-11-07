// Copyright (C) 2018-Present Pivotal Software, Inc. All rights reserved.
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
	. "github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("getting bosh configs", func() {
	var (
		configType      = "some-config-type"
		configName      = "some-config-name"
		configContent   = "some-config-content"
		directorConfigs []director.Config
		boshConfigs     []BoshConfig
		listConfigsErr  error
	)

	BeforeEach(func() {
		directorConfigs = []director.Config{
			director.Config{
				ID:      "some-config-id",
				Type:    configType,
				Name:    configName,
				Content: configContent,
				Team:    "some-config-team",
			},
		}
	})

	Describe("GetConfigs", func() {
		It("returns the bosh configs", func() {
			fakeDirector.ListConfigsReturns(directorConfigs, nil)
			boshConfigs, listConfigsErr = c.GetConfigs(configName, logger)

			Expect(boshConfigs).To(Equal([]BoshConfig{
				BoshConfig{
					Type:    configType,
					Name:    configName,
					Content: configContent,
				},
			}))
			Expect(listConfigsErr).NotTo(HaveOccurred())
		})

		It("returns an error when the client cannot list configs", func() {
			fakeDirector.ListConfigsReturns([]director.Config{}, errors.New("oops"))
			boshConfigs, listConfigsErr = c.GetConfigs(configName, logger)

			Expect(listConfigsErr).To(MatchError(ContainSubstring(`BOSH error getting configs for "some-config-name"`)))
		})
	})
})
