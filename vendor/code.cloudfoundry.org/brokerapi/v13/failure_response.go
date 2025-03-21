// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package brokerapi

import (
	"code.cloudfoundry.org/brokerapi/v13/domain/apiresponses"
)

// Deprecated: Use code.cloudfoundry.org/brokerapi/v13/domain/apiresponses
// FailureResponse can be returned from any of the `ServiceBroker` interface methods
// which allow an error to be returned. Doing so will provide greater control over
// the HTTP response.
type FailureResponse = apiresponses.FailureResponse

// Deprecated: Use code.cloudfoundry.org/brokerapi/v13/domain/apiresponses
// NewFailureResponse returns an error of type FailureResponse.
// err will by default be used as both a logging message and HTTP response description.
// statusCode is the HTTP status code to be returned, must be 4xx or 5xx
// loggerAction is a short description which will be used as the action if the error is logged.
func NewFailureResponse(err error, statusCode int, loggerAction string) error {
	return apiresponses.NewFailureResponse(err, statusCode, loggerAction)
}

// Deprecated: Use code.cloudfoundry.org/brokerapi/v13/domain/apiresponses
// FailureResponseBuilder provides a fluent set of methods to build a *FailureResponse.
type FailureResponseBuilder = apiresponses.FailureResponseBuilder

// Deprecated: Use code.cloudfoundry.org/brokerapi/v13/domain/apiresponses
// NewFailureResponseBuilder returns a pointer to a newly instantiated FailureResponseBuilder
// Accepts required arguments to create a FailureResponse.
func NewFailureResponseBuilder(err error, statusCode int, loggerAction string) *FailureResponseBuilder {
	return (*FailureResponseBuilder)(apiresponses.NewFailureResponseBuilder(err, statusCode, loggerAction))
}
