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
)

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

		It("returns an error when the client cannot get the latest config", func() {
			fakeDirector.UpdateConfigReturns(director.Config{}, errors.New("oops"))
			updateConfigErr = c.UpdateConfig(configType, configName, []byte(configContent), logger)

			Expect(updateConfigErr).To(MatchError(ContainSubstring(`BOSH error updating "some-config-type" config "some-config-name"`)))
		})
	})
})
