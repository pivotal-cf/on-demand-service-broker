// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockbroker

import (
	"fmt"

	"net/url"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

type lastOperationMock struct {
	*mockhttp.MockHttp
}

func LastOperation(serviceInstanceGUID, operationData string) *lastOperationMock {
	operationQuery := url.QueryEscape(operationData)
	path := fmt.Sprintf("/v2/service_instances/%s/last_operation?operation=%s", serviceInstanceGUID, operationQuery)
	return &lastOperationMock{
		mockhttp.NewMockedHttpRequest("GET", path),
	}
}

func (l *lastOperationMock) RespondWithOperationSucceeded() *mockhttp.MockHttp {
	operationSucceed := brokerapi.LastOperation{
		State:       brokerapi.Succeeded,
		Description: "its done!",
	}

	return l.RespondsWithJson(operationSucceed)
}

func (l *lastOperationMock) RespondWithOperationInProgress() *mockhttp.MockHttp {
	operationInProgress := brokerapi.LastOperation{
		State:       brokerapi.InProgress,
		Description: "its done!",
	}

	return l.RespondsWithJson(operationInProgress)
}
