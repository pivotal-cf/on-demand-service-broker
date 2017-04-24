// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package create_test

import (
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"

	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("Creates service instances", func() {
	It("Successfully creates all instances", func() {
		By("Issuing create requests")
		for i := 0; i < instances; i++ {
			instance := fmt.Sprintf("load-test-%v", i)
			Eventually(cf.Cf("cs", service, plan, instance), cf_helpers.CfTimeout).Should(gexec.Exit(0))
			time.Sleep(time.Duration(interval) * time.Second)
		}

		By("waiting for creation to complete")
		for i := 0; i < instances; i++ {
			name := fmt.Sprintf("load-test-%v", i)
			//Creating 500 instances can take 10 hours. So set a long timeout!
			cf_helpers.AwaitServiceCreationWithTimeout(name, time.Duration(24)*time.Hour)
		}
	})
})
