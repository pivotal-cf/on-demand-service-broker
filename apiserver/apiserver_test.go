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
	"net/http"
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
	It("delegates requests through to the regular broker when credhub isn't configured", func() {
		fakeBroker := new(fakes.FakeCombinedBrokers)

		username := "admin"
		password := "admin"

		conf := config.Config{
			Broker: config.Broker{
				Port:                   12345,
				StartUpBanner:          true,
				DisableCFStartupChecks: true,
				Username:               username,
				Password:               password,
			},
		}
		server := createServer(conf, fakeBroker, nil)

		instanceID := "foo-bar"
		bindingID := "12345"
		route := fmt.Sprintf("http://localhost/v2/service_instances/%s/service_bindings/%s", instanceID, bindingID)

		resp := put(server, route, username, password, brokerapi.BindDetails{
			AppGUID:   "",
			PlanID:    "",
			ServiceID: "",
		})

		Expect(resp.StatusCode).To(Equal(201))
		Expect(fakeBroker.BindCallCount()).To(Equal(1))
		_, receivedInstanceID, _, _ := fakeBroker.BindArgsForCall(0)
		Expect(receivedInstanceID).To(Equal(instanceID))
	})

	It("delegates requests to the credhub broker when credhub is configured", func() {
		regularBroker := new(fakes.FakeCombinedBrokers)
		credhubBroker := new(fakes.FakeCombinedBrokers)

		username := "admin"
		password := "admin"

		conf := config.Config{
			Broker: config.Broker{
				Port:                   12345,
				StartUpBanner:          true,
				DisableCFStartupChecks: true,
				Username:               username,
				Password:               password,
			},
			CredHub: config.CredHub{
				APIURL:       "https://local.foo.host:8844",
				ClientID:     "a",
				ClientSecret: "b8",
			},
		}
		server := createServer(conf, regularBroker, credhubBroker)

		instanceID := "foo-bar"
		bindingID := "12345"
		route := fmt.Sprintf("http://localhost/v2/service_instances/%s/service_bindings/%s", instanceID, bindingID)

		resp := put(server, route, username, password, brokerapi.BindDetails{
			AppGUID:   "",
			PlanID:    "",
			ServiceID: "",
		})

		Expect(resp.StatusCode).To(Equal(201))
		Expect(regularBroker.BindCallCount()).To(Equal(0))
		Expect(credhubBroker.BindCallCount()).To(Equal(1))
		_, receivedInstanceID, _, _ := credhubBroker.BindArgsForCall(0)
		Expect(receivedInstanceID).To(Equal(instanceID))

	})
})

func put(server *http.Server, route string, username, password string, body interface{}) *http.Response {
	reqBody, err := json.Marshal(body)
	Expect(err).NotTo(HaveOccurred())
	req := httptest.NewRequest("PUT", route, bytes.NewReader(reqBody))
	req.SetBasicAuth(username, password)
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	return w.Result()
}

func createServer(conf config.Config, baseBroker, credhubBroker apiserver.CombinedBrokers) *http.Server {
	mgmtapiLoggerFactory := loggerfactory.New(GinkgoWriter, "my loggerz", 0)
	serverLogger := log.New(GinkgoWriter, "test output: ", 0)
	return apiserver.New(conf, baseBroker, credhubBroker, "the-best-broker", mgmtapiLoggerFactory, serverLogger)
}
