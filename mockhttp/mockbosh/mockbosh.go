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

const (
	BoshContextIDHeader = "X-Bosh-Context-Id"
	serverName          = "mock-bosh"
)

type MockBOSH struct {
	*mockhttp.Server
	UAAURL string
}

func NewWithUAA(uaaUrl string) *MockBOSH {
	certPath := pathToSSLFixtures("cert.pem")
	keyPath := pathToSSLFixtures("key.pem")
	return &MockBOSH{
		UAAURL: uaaUrl,
		Server: mockhttp.StartTLSServer(serverName, certPath, keyPath),
	}
}

func New() *MockBOSH {
	certPath := pathToSSLFixtures("cert.pem")
	keyPath := pathToSSLFixtures("key.pem")
	return &MockBOSH{Server: mockhttp.StartTLSServer(serverName, certPath, keyPath)}
}

func pathToSSLFixtures(filename string) string {
	return fmt.Sprintf("../../integration_tests/fixtures/ssl/%s", filename)
}
