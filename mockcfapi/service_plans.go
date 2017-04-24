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

	"github.com/pivotal-cf/on-demand-service-broker/cloud_foundry_client"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

type servicePlansMock struct {
	*mockhttp.MockHttp
}

func GetServicePlan(servicePlanGUID string) *mockhttp.MockHttp {
	return mockhttp.NewMockedHttpRequest("GET", "/v2/service_plans/"+servicePlanGUID)
}

func ListServicePlans(serviceID string) *servicePlansMock {
	return &servicePlansMock{
		mockhttp.NewMockedHttpRequest("GET", "/v2/services/"+serviceID+"/service_plans?results-per-page=100"),
	}
}

func ListServicePlansForPage(serviceID string, page int) *servicePlansMock {
	return &servicePlansMock{
		mockhttp.NewMockedHttpRequest("GET", fmt.Sprintf("/v2/services/%s/service_plans?order-direction=asc&page=%d&results-per-page=100", serviceID, page)),
	}
}

func (m *servicePlansMock) RespondsWithServicePlan(planID, cloudControllerGUID string) *mockhttp.MockHttp {
	return m.RespondsWith(listServicePlansResponse(Plan{ID: planID, CloudControllerGUID: cloudControllerGUID}))
}

func (m *servicePlansMock) RespondsWithServicePlans(plans ...Plan) *mockhttp.MockHttp {
	return m.RespondsWith(listServicePlansResponse(plans...))
}

func (m *servicePlansMock) RespondsWithNoServicePlans() *mockhttp.MockHttp {
	return m.RespondsWith(listServicePlansResponse())
}

type Plan struct {
	ID                  string
	CloudControllerGUID string
}

func listServicePlansResponse(plans ...Plan) string {
	servicePlans := []cloud_foundry_client.ServicePlan{}

	for _, plan := range plans {
		servicePlans = append(servicePlans, cloud_foundry_client.ServicePlan{
			Metadata: cloud_foundry_client.Metadata{GUID: plan.CloudControllerGUID},
			ServicePlanEntity: cloud_foundry_client.ServicePlanEntity{
				UniqueID:            plan.ID,
				ServiceInstancesUrl: "/v2/service_plans/" + plan.CloudControllerGUID + "/service_instances",
			},
		})
	}

	response, err := json.Marshal(cloud_foundry_client.ServicePlanResponse{ServicePlans: servicePlans})
	Expect(err).NotTo(HaveOccurred())
	return string(response)
}
