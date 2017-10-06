// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockbosh

import (
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

type errandMock struct {
	*mockhttp.Handler
}

func Errand(deploymentName, errandName, body string) *errandMock {
	mock := errandMock{
		Handler: mockhttp.NewMockedHttpRequest("POST", fmt.Sprintf("/deployments/%s/errands/%s/runs", deploymentName, errandName)),
	}
	mock.WithContentType("application/json")
	mock.WithJSONBody(body)
	return &mock
}

func (e *errandMock) WithAnyContextID() *errandMock {
	e.WithHeaderPresent(BoshContextIDHeader)
	return e
}

func (e *errandMock) WithContextID(value string) *errandMock {
	e.WithHeader(BoshContextIDHeader, value)
	return e
}

func (e *errandMock) WithoutContextID() *errandMock {
	e.WithoutHeader(BoshContextIDHeader)
	return e
}

func (e *errandMock) RedirectsToTask(taskID int) *mockhttp.Handler {
	return e.RedirectsTo(taskURL(taskID))
}
