// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockcredhub

import "github.com/pivotal-cf/on-demand-service-broker/mockhttp"

type putCredentialMock struct {
	*mockhttp.Handler
	identifier string
}

func PutCredential(identifier string) *putCredentialMock {
	return &putCredentialMock{
		Handler:    mockhttp.NewMockedHttpRequest("PUT", "/api/v1/data"),
		identifier: identifier,
	}
}

func (m *putCredentialMock) WithPassword(password string) *putCredentialMock {
	decoratedMock := m.WithJSONBody(map[string]interface{}{
		"name":      m.identifier,
		"type":      "password",
		"value":     password,
		"overwrite": false,
	})
	return &putCredentialMock{
		Handler:    decoratedMock,
		identifier: m.identifier,
	}
}

func (m *putCredentialMock) RespondsWithPasswordData(password string) *mockhttp.Handler {
	body := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":         m.identifier,
				"type":       "password",
				"value":      password,
				"updated_at": "2016-01-01T12:00:00Z",
			},
		},
	}

	return m.RespondsOKWithJSON(body)
}
