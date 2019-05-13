// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockcfapi

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

type servicePlansMock struct {
	*mockhttp.Handler
}

func GetServicePlan(servicePlanGUID string) *mockhttp.Handler {
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

func ListServicePlanVisibilities(planGUID string) *servicePlansMock {
	return &servicePlansMock{
		mockhttp.NewMockedHttpRequest("GET", "/v2/service_plan_visibilities?q=service_plan_guid:"+planGUID+"&results-per-page=100"),
	}
}

func DeleteServicePlanVisibility(visibilityGUID string) *servicePlansMock {
	return &servicePlansMock{
		mockhttp.NewMockedHttpRequest("DELETE", "/v2/service_plan_visibilities/"+visibilityGUID),
	}
}

func DisablePlanAccess(planID string) *servicePlansMock {
	return PutPlanAccess(planID, `{"public":false}`)
}

func EnablePlanAccess(planID string) *servicePlansMock {
	return PutPlanAccess(planID, `{"public":true}`)
}

func PutPlanAccess(planID, body string) *servicePlansMock {
	mock := &servicePlansMock{
		mockhttp.NewMockedHttpRequest("PUT", fmt.Sprintf("/v2/service_plans/%s", planID)),
	}

	mock.WithContentType("application/x-www-form-urlencoded")
	mock.WithBody(body)

	return mock
}

func (m *servicePlansMock) RespondsWithServicePlan(planID, cloudControllerGUID string) *mockhttp.Handler {
	return m.RespondsOKWith(listServicePlansResponse(Plan{ID: planID, CloudControllerGUID: cloudControllerGUID}))
}

func (m *servicePlansMock) RespondsWithServicePlans(plans ...Plan) *mockhttp.Handler {
	return m.RespondsOKWith(listServicePlansResponse(plans...))
}

func (m *servicePlansMock) RespondsWithNoServicePlans() *mockhttp.Handler {
	return m.RespondsOKWith(listServicePlansResponse())
}

type Plan struct {
	ID                  string
	CloudControllerGUID string
}

func listServicePlansResponse(plans ...Plan) string {
	var servicePlans []cf.ServicePlan

	for _, plan := range plans {
		servicePlans = append(servicePlans, cf.ServicePlan{
			Metadata: cf.Metadata{GUID: plan.CloudControllerGUID},
			ServicePlanEntity: cf.ServicePlanEntity{
				UniqueID:            plan.ID,
				ServiceInstancesUrl: "/v2/service_plans/" + plan.CloudControllerGUID + "/service_instances",
			},
		})
	}

	response, err := json.Marshal(cf.ServicePlanResponse{ServicePlans: servicePlans})
	Expect(err).NotTo(HaveOccurred())
	return string(response)
}
