// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockbosh

import "github.com/pivotal-cf/on-demand-service-broker/mockhttp"

type infoMock struct {
	*mockhttp.MockHttp
}

func Info() *infoMock {
	return &infoMock{
		mockhttp.NewMockedHttpRequest("GET", "/info"),
	}
}

func (m *infoMock) RespondsWithSufficientStemcellVersionForODB() *mockhttp.MockHttp {
	return m.RespondsWith(`{"version":"1.3262.0.0 (00000000)"}`)
}

func (m *infoMock) RespondsWithSufficientSemverVersionForODB() *mockhttp.MockHttp {
	return m.RespondsWith(`{"version":"260.0.0 (00000000)"}`)
}

func (m *infoMock) RespondsWithSufficientVersionForLifecycleErrands() *mockhttp.MockHttp {
	return m.RespondsWith(`{"version":"261.0.0 (00000000)"}`)
}
