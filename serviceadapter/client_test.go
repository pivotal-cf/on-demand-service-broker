// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package serviceadapter_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

var _ = Describe("Adapter Client", func() {
	DescribeTable("ErrorForExitCode",
		func(code int, msg string, matchErr, matchMsg types.GomegaMatcher) {
			err := serviceadapter.ErrorForExitCode(code, msg)
			Expect(err).To(matchErr)

			if err != nil {
				Expect(err.Error()).To(matchMsg)
			}
		},
		Entry(
			"success",
			serviceadapter.SuccessExitCode, "",
			BeNil(),
			nil,
		),
		Entry(
			"not implemented",
			sdk.NotImplementedExitCode, "should not appear",
			BeAssignableToTypeOf(serviceadapter.NotImplementedError{}),
			Equal("command not implemented by service adapter"),
		),
		Entry(
			"app guid not provided",
			sdk.AppGuidNotProvidedErrorExitCode, "should not appear",
			BeAssignableToTypeOf(serviceadapter.AppGuidNotProvidedError{}),
			Equal("app GUID not provided"),
		),
		Entry(
			"binding already exists",
			sdk.BindingAlreadyExistsErrorExitCode, "should not appear",
			BeAssignableToTypeOf(serviceadapter.BindingAlreadyExistsError{}),
			Equal("binding already exists"),
		),
		Entry(
			"binding not found",
			sdk.BindingNotFoundErrorExitCode, "should not appear",
			BeAssignableToTypeOf(serviceadapter.BindingNotFoundError{}),
			Equal("binding not found"),
		),
		Entry(
			"standard error exit code",
			sdk.ErrorExitCode, "some error",
			BeAssignableToTypeOf(serviceadapter.UnknownFailureError{}),
			Equal("some error"),
		),
		Entry(
			"some other non-zero exit code",
			12345, "some other error",
			BeAssignableToTypeOf(serviceadapter.UnknownFailureError{}),
			Equal("some other error"),
		),
	)
})
