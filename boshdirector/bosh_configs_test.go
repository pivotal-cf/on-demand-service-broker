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
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("getting bosh configs", func() {
	var (
		configType      = "some-config-type"
		configName      = "some-config-name"
		configContent   = "some-config-content"
		directorConfigs []director.Config
		boshConfigs     []boshdirector.BoshConfig
		listConfigsErr  error
	)

	BeforeEach(func() {
		directorConfigs = []director.Config{
			{
				ID:      "some-config-id",
				Type:    configType,
				Name:    configName,
				Content: configContent,
				Team:    "some-config-team",
			},
			{
				ID:      "some-other-config-id",
				Type:    configType + "-2nd",
				Name:    configName + "-2nd",
				Content: configContent + "-2nd",
				Team:    "some-other-config-team",
			},
		}
	})

	Describe("GetConfigs", func() {
		It("returns the bosh configs", func() {
			fakeDirector.ListConfigsReturns(directorConfigs, nil)
			boshConfigs, listConfigsErr = c.GetConfigs(configName, logger)

			Expect(boshConfigs).To(Equal([]boshdirector.BoshConfig{
				{
					Type:    configType,
					Name:    configName,
					Content: configContent,
				},
				{
					Type:    configType + "-2nd",
					Name:    configName + "-2nd",
					Content: configContent + "-2nd",
				},
			}))
			Expect(listConfigsErr).NotTo(HaveOccurred())
		})

		It("returns an error when the director can't be built", func() {
			fakeDirectorFactory.NewReturns(nil, errors.New("can't get director"))
			_, listConfigsErr = c.GetConfigs(configName, logger)

			Expect(listConfigsErr).To(HaveOccurred())
			Expect(listConfigsErr).To(MatchError(ContainSubstring("Failed to build director: can't get director")))
		})

		It("returns an error when the client cannot list configs", func() {
			fakeDirector.ListConfigsReturns([]director.Config{}, errors.New("oops"))
			_, listConfigsErr = c.GetConfigs(configName, logger)

			Expect(listConfigsErr).To(HaveOccurred())
			Expect(listConfigsErr).To(MatchError(ContainSubstring(`BOSH error getting configs for "some-config-name"`)))
		})
	})
})

var _ = Describe("updating bosh config", func() {
	var (
		configType      = "some-config-type"
		configName      = "some-config-name"
		configContent   = "some-config-content"
		updateConfigErr error
	)

	Describe("UpdateConfig", func() {
		It("returns the bosh config when the latest config exists", func() {
			fakeDirector.UpdateConfigReturns(director.Config{}, nil)
			updateConfigErr = c.UpdateConfig(configType, configName, []byte(configContent), logger)

			Expect(updateConfigErr).NotTo(HaveOccurred())
		})

		It("returns an error when the director can't be built", func() {
			fakeDirectorFactory.NewReturns(nil, errors.New("can't get director"))
			updateConfigErr = c.UpdateConfig(configType, configName, []byte(configContent), logger)

			Expect(updateConfigErr).To(HaveOccurred())
			Expect(updateConfigErr).To(MatchError(ContainSubstring("Failed to build director: can't get director")))
		})

		It("returns an error when the client cannot get the latest config", func() {
			fakeDirector.UpdateConfigReturns(director.Config{}, errors.New("oops"))
			updateConfigErr = c.UpdateConfig(configType, configName, []byte(configContent), logger)

			Expect(updateConfigErr).To(HaveOccurred())
			Expect(updateConfigErr).To(MatchError(ContainSubstring(`BOSH error updating "some-config-type" config "some-config-name"`)))
		})
	})
})

var _ = Describe("deleting bosh config", func() {
	var (
		configType      = "some-config-type"
		configName      = "some-config-name"
		configFound     bool
		deleteConfigErr error
	)

	Describe("DeleteConfig", func() {
		It("returns true when the config exists", func() {
			fakeDirector.DeleteConfigReturns(true, nil)
			configFound, deleteConfigErr = c.DeleteConfig(configType, configName, logger)

			Expect(configFound).To(BeTrue())
			Expect(deleteConfigErr).NotTo(HaveOccurred())
		})

		It("returns false when the config does not exists", func() {
			fakeDirector.DeleteConfigReturns(false, nil)
			configFound, deleteConfigErr = c.DeleteConfig(configType, configName, logger)

			Expect(configFound).To(BeFalse())
			Expect(deleteConfigErr).NotTo(HaveOccurred())
		})

		It("returns an error when the director can't be built", func() {
			fakeDirectorFactory.NewReturns(nil, errors.New("can't get director"))
			_, deleteConfigErr = c.DeleteConfig(configType, configName, logger)

			Expect(deleteConfigErr).To(HaveOccurred())
			Expect(deleteConfigErr).To(MatchError(ContainSubstring("Failed to build director: can't get director")))
		})

		It("returns an error when the client cannot delete the config", func() {
			fakeDirector.DeleteConfigReturns(false, errors.New("oops"))
			_, deleteConfigErr = c.DeleteConfig(configType, configName, logger)

			Expect(deleteConfigErr).To(HaveOccurred())
			Expect(deleteConfigErr).To(MatchError(ContainSubstring(`BOSH error deleting "some-config-type" config "some-config-name"`)))
		})
	})
})

var _ = Describe("deleting bosh configs", func() {
	var (
		configType       = "some-config-type"
		configName       = "some-config-name"
		configContent    = "some-config-content"
		directorConfigs  []director.Config
		deleteConfigsErr error
	)

	BeforeEach(func() {
		directorConfigs = []director.Config{
			{
				ID:      "some-config-id",
				Type:    configType,
				Name:    configName,
				Content: configContent,
				Team:    "some-config-team",
			},
			{
				ID:      "some-other-config-id",
				Type:    configType + "-2nd",
				Name:    configName + "-2nd",
				Content: configContent + "-2nd",
				Team:    "some-other-config-team",
			},
		}
	})

	Describe("DeleteConfigs", func() {
		It("deletes all configs", func() {
			fakeDirector.ListConfigsReturns(directorConfigs, nil)
			deleteConfigsErr = c.DeleteConfigs(configName, logger)

			Expect(deleteConfigsErr).NotTo(HaveOccurred())
			Expect(fakeDirector.DeleteConfigCallCount()).To(Equal(2))
		})

		It("does not delete any config when there are not any bosh configs", func() {
			fakeDirector.ListConfigsReturns([]director.Config{}, nil)
			deleteConfigsErr = c.DeleteConfigs(configName, logger)

			Expect(deleteConfigsErr).NotTo(HaveOccurred())
			Expect(fakeDirector.DeleteConfigCallCount()).To(Equal(0))
		})

		It("returns an error when the director can't be built", func() {
			fakeDirectorFactory.NewReturns(nil, errors.New("can't get director"))
			deleteConfigsErr = c.DeleteConfigs(configName, logger)

			Expect(deleteConfigsErr).To(HaveOccurred())
			Expect(deleteConfigsErr).To(MatchError(ContainSubstring("Failed to build director: can't get director")))
		})

		It("returns an error when the client cannot list configs", func() {
			fakeDirector.ListConfigsReturns([]director.Config{}, errors.New("oops"))
			deleteConfigsErr = c.DeleteConfigs(configName, logger)

			Expect(deleteConfigsErr).To(HaveOccurred())
			Expect(deleteConfigsErr).To(MatchError(ContainSubstring(`BOSH error getting configs for "some-config-name`)))
		})

		It("rreturns an error when the client cannot delete the config", func() {
			fakeDirector.ListConfigsReturns(directorConfigs, nil)
			fakeDirector.DeleteConfigReturns(false, errors.New("oops"))
			deleteConfigsErr = c.DeleteConfigs(configName, logger)

			Expect(deleteConfigsErr).To(HaveOccurred())
			Expect(deleteConfigsErr).To(MatchError(ContainSubstring(`BOSH error deleting "some-config-type" config "some-config-name"`)))
		})
	})
})
