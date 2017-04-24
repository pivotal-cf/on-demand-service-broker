// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockcfapi

import (
	"fmt"

	. "github.com/onsi/gomega"

	"encoding/json"

	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

type serviceInstancesMock struct {
	*mockhttp.MockHttp
}

func ListServiceInstances(servicePlanGUID string) *serviceInstancesMock {
	return &serviceInstancesMock{

		mockhttp.NewMockedHttpRequest(
			"GET",
			"/v2/service_plans/"+servicePlanGUID+"/service_instances?results-per-page=100",
		),
	}
}

func ListServiceInstancesForPage(servicePlanGUID string, page int) *serviceInstancesMock {
	return &serviceInstancesMock{
		mockhttp.NewMockedHttpRequest(
			"GET",
			fmt.Sprintf(
				"/v2/service_plans/%s/service_instances?order-direction=asc&page=%d&results-per-page=100",
				servicePlanGUID,
				page),
		),
	}
}

func DeleteServiceInstance(instanceGUID string) *serviceInstancesMock {
	path := fmt.Sprintf("/v2/service_instances/%s?accepts_incomplete=true", instanceGUID)
	return &serviceInstancesMock{mockhttp.NewMockedHttpRequest("DELETE", path)}
}

func GetServiceInstance(serviceInstanceGUID string) *serviceInstancesMock {
	return &serviceInstancesMock{
		mockhttp.NewMockedHttpRequest("GET", "/v2/service_instances/"+serviceInstanceGUID),
	}
}

func (m *serviceInstancesMock) RespondsWithDeleteInProgress(instanceGUID string) *mockhttp.MockHttp {
	body := fmt.Sprintf(instanceResponseBody, instanceGUID, "in progress")
	return m.RespondsWith(body)
}

func (m *serviceInstancesMock) RespondsWithDeleteFailed(instanceGUID string) *mockhttp.MockHttp {
	body := fmt.Sprintf(instanceResponseBody, instanceGUID, "failed")
	return m.RespondsWith(body)
}

func (m *serviceInstancesMock) RespondsWithNoServiceInstances() *mockhttp.MockHttp {
	return m.RespondsWith(`{
			"total_results": 1,
			"total_pages": 1,
			"prev_url": null,
			"next_url": null,
			"resources": []
		}`)
}

func (m *serviceInstancesMock) RespondsWithServiceInstances(instanceIDs ...string) *mockhttp.MockHttp {
	return m.RespondsWith(listServiceInstancesResponse(instanceIDs...))
}

func (m *serviceInstancesMock) RespondsWithPaginatedServiceInstances(
	planID string,
	page,
	resultsPerPage,
	totalPages int,
	instanceIDs ...string,
) *mockhttp.MockHttp {
	return m.RespondsWith(paginatedListServiceInstanceResponse(
		planID,
		page,
		resultsPerPage,
		totalPages,
		instanceIDs...,
	))
}

func listServiceInstancesResponse(instanceIDs ...string) string {
	resources := []serviceInstanceResource{}
	for _, instanceID := range instanceIDs {
		resources = append(resources, serviceInstanceResource{Metadata: serviceInstanceMetadata{GUID: instanceID}})
	}

	responseBytes, err := json.Marshal(serviceInstances{Resources: resources})
	Expect(err).NotTo(HaveOccurred())
	return string(responseBytes)
}

func paginatedListServiceInstanceResponse(
	planID string,
	page,
	resultsPerPage,
	totalPages int,
	instanceIDs ...string,
) string {
	Expect(len(instanceIDs)).To(BeNumerically("<=", resultsPerPage))
	var nextURL, prevURL string

	if page < totalPages {
		nextURL = fmt.Sprintf(
			"/v2/service_plans/%s/service_instances?order-direction=asc&page=%d&results-per-page=%d",
			planID,
			page+1,
			resultsPerPage,
		)
	}

	if page > 1 {
		prevURL = fmt.Sprintf(
			"/v2/service_plans/%s/service_instances?order-direction=asc&page=%d&results-per-page=%d",
			planID,
			page-1,
			resultsPerPage,
		)
	}

	resources := []serviceInstanceResource{}
	for _, instanceID := range instanceIDs {
		resources = append(resources, serviceInstanceResource{Metadata: serviceInstanceMetadata{GUID: instanceID}})
	}

	responseBytes, err := json.Marshal(serviceInstances{Resources: resources, NextURL: &nextURL, PrevURL: &prevURL})
	Expect(err).NotTo(HaveOccurred())

	return string(responseBytes)
}

type serviceInstances struct {
	NextURL   *string                   `json:"next_url"`
	PrevURL   *string                   `json:"prev_url"`
	Resources []serviceInstanceResource `json:"resources"`
}

type serviceInstanceResource struct {
	Metadata serviceInstanceMetadata `json:"metadata"`
}

type serviceInstanceMetadata struct {
	GUID string `json:"guid"`
}

var instanceResponseBody string = `{
	"metadata": {
		"guid": "%s"
	},
	"entity": {
		"last_operation": {
			"type": "delete",
			"state": "%s"
		}
	}
}`
