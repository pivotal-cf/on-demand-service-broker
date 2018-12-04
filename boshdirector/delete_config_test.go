// Copyright (C) 2018-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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

		It("returns an error when the client cannot delete the config", func() {
			fakeDirector.DeleteConfigReturns(false, errors.New("oops"))
			configFound, deleteConfigErr = c.DeleteConfig(configType, configName, logger)

			Expect(configFound).To(BeFalse())
			Expect(deleteConfigErr).To(MatchError(ContainSubstring(`BOSH error deleting "some-config-type" config "some-config-name"`)))
		})
	})
})
