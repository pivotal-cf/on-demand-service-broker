// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package apiserver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

var _ = Describe("API server", func() {
	It("delegates requests through to the regular broker", func() {
		fakeBroker := new(fakes.FakeCombinedBrokers)

		port := 12345
		username := "admin"
		password := "admin"

		conf := config.Config{
			Broker: config.Broker{
				Port:                   port,
				StartUpBanner:          true,
				DisableCFStartupChecks: true,
				Username:               username,
				Password:               password,
			},
		}
		loggerFac := loggerfactory.New(GinkgoWriter, "my loggerz", 0)
		logger := log.New(GinkgoWriter, "test output: ", 0)
		server := apiserver.New(conf, fakeBroker, "the-best-broker", loggerFac, logger)

		instanceID := "foo-bar"
		bindingID := "12345"
		route := fmt.Sprintf("http://localhost/v2/service_instances/%s/service_bindings/%s", instanceID, bindingID)
		bindDetails := brokerapi.BindDetails{
			AppGUID:   "",
			PlanID:    "",
			ServiceID: "",
		}

		reqBody, err := json.Marshal(bindDetails)
		Expect(err).NotTo(HaveOccurred())

		req := httptest.NewRequest("PUT", route, bytes.NewReader(reqBody))
		req.SetBasicAuth(username, password)

		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, req)
		resp := w.Result()

		Expect(resp.StatusCode).To(Equal(201))
		Expect(fakeBroker.BindCallCount()).To(Equal(1))
		_, receivedInstanceID, _, _ := fakeBroker.BindArgsForCall(0)
		Expect(receivedInstanceID).To(Equal(instanceID))
	})
})
