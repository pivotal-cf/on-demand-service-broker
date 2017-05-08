// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package adapterclient_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/pivotal-cf/on-demand-service-broker/adapterclient"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Adapter Client", func() {

	DescribeTable("ErrorForExitCode",
		func(code int, msg string, matchErr types.GomegaMatcher, matchMsg types.GomegaMatcher) {
			err := adapterclient.ErrorForExitCode(code, msg)
			Expect(err).To(matchErr)

			if err != nil {
				Expect(err.Error()).To(matchMsg)
			}
		},
		Entry(
			"success",
			adapterclient.SuccessExitCode, "",
			BeNil(),
			nil,
		),
		Entry(
			"not implemented",
			serviceadapter.NotImplementedExitCode, "should not appear",
			BeAssignableToTypeOf(adapterclient.NotImplementedError{}),
			Equal("command not implemented by service adapter"),
		),
		Entry(
			"app guid not provided",
			serviceadapter.AppGuidNotProvidedErrorExitCode, "should not appear",
			BeAssignableToTypeOf(adapterclient.AppGuidNotProvidedError{}),
			Equal("app GUID not provided"),
		),
		Entry(
			"binding already exists",
			serviceadapter.BindingAlreadyExistsErrorExitCode, "should not appear",
			BeAssignableToTypeOf(adapterclient.BindingAlreadyExistsError{}),
			Equal("binding already exists"),
		),
		Entry(
			"binding not found",
			serviceadapter.BindingNotFoundErrorExitCode, "should not appear",
			BeAssignableToTypeOf(adapterclient.BindingNotFoundError{}),
			Equal("binding not found"),
		),
		Entry(
			"standard error exit code",
			serviceadapter.ErrorExitCode, "some error",
			BeAssignableToTypeOf(adapterclient.UnknownFailureError{}),
			Equal("some error"),
		),
		Entry(
			"some other non-zero exit code",
			12345, "some other error",
			BeAssignableToTypeOf(adapterclient.UnknownFailureError{}),
			Equal("some other error"),
		),
	)
})
