// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockcfapi

import (
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

type serviceBindingsMock struct {
	*mockhttp.Handler
}

func ListServiceBindings(serviceInstanceGUID string) *serviceBindingsMock {
	return &serviceBindingsMock{
		mockhttp.NewMockedHttpRequest("GET", "/v2/service_instances/"+serviceInstanceGUID+"/service_bindings?results-per-page=100"),
	}
}

func ListServiceBindingsForPage(serviceInstanceGUID string, page int) *serviceBindingsMock {
	return &serviceBindingsMock{
		mockhttp.NewMockedHttpRequest("GET",
			fmt.Sprintf(
				"/v2/service_instances/%s/service_bindings?order-direction=asc&page=%d&results-per-page=100",
				serviceInstanceGUID,
				page),
		),
	}
}

func DeleteServiceBinding(appGUID, bindingGUID string) *mockhttp.Handler {
	path := fmt.Sprintf("/v2/apps/%s/service_bindings/%s", appGUID, bindingGUID)
	return mockhttp.NewMockedHttpRequest("DELETE", path)
}

func (m *serviceBindingsMock) RespondsWithServiceBinding(bindingGUID, instanceGUID, appGUID string) *mockhttp.Handler {
	return m.RespondsOKWith(fmt.Sprintf(`{
		"total_results": 1,
		"total_pages": 1,
		"prev_url": null,
		"next_url": null,
		"resources": [
			{
				"metadata": {
					"guid": "%s"
				},
				"entity": {
					"service_instance_guid": "%s",
					"app_guid": "%s"
				}
			}
		]
	}`, bindingGUID, instanceGUID, appGUID))
}
