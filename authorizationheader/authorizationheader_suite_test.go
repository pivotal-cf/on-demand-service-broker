// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package authorizationheader_test

import (
	"fmt"
	"log"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAuthorizationHeader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Authorization Header Suite")
}

var logger *log.Logger

var _ = BeforeSuite(func() {
	logger = log.New(GinkgoWriter, "[authorizationheader unit test]", log.LstdFlags)
})

func pathToSSLCerts(filename string) string {
	return fmt.Sprintf("../integration_tests/fixtures/ssl/%s", filename)
}
