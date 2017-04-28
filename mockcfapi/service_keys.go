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

type serviceKeysMock struct {
	*mockhttp.Handler
}

func ListServiceKeys(serviceInstanceGUID string) *serviceKeysMock {
	return &serviceKeysMock{
		mockhttp.NewMockedHttpRequest("GET", "/v2/service_instances/"+serviceInstanceGUID+"/service_keys?results-per-page=100"),
	}
}

func DeleteServiceKey(serviceKeyGUID string) *mockhttp.Handler {
	path := fmt.Sprintf("/v2/service_keys/%s", serviceKeyGUID)
	return mockhttp.NewMockedHttpRequest("DELETE", path)
}

func ListServiceKeysForPage(serviceInstanceGUID string, page int) *serviceKeysMock {
	return &serviceKeysMock{
		mockhttp.NewMockedHttpRequest("GET",
			fmt.Sprintf(
				"/v2/service_instances/%s/service_keys?order-direction=asc&page=%d&results-per-page=100",
				serviceInstanceGUID,
				page),
		),
	}
}

func (m *serviceKeysMock) RespondsWithServiceKey(serviceKeyGUID, instanceGUID string) *mockhttp.Handler {
	return m.RespondsOKWith(fmt.Sprintf(`{
						"total_results": 1,
						"total_pages": 1,
						"prev_url": null,
						"next_url": null,
						"resources": [
							{
								"metadata": {
									"guid": "%s",
									"url": "/v2/service_keys/%s"
								},
								"entity": {
									"service_instance_guid": "%s"
								}
							}
						]
					}`, serviceKeyGUID, serviceKeyGUID, instanceGUID))
}
