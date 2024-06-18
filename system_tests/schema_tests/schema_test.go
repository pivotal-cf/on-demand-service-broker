// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema_tests

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/brokerapi/v11/domain"

	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

type CFServices struct {
	Resources []struct {
		Entity struct {
			ServicePlansURL string `json:"service_plans_url"`
		} `json:"entity"`
	} `json:"resources"`
}

type CFServicePlans struct {
	Resources []struct {
		Entity domain.ServicePlan
	}
}

var _ = Describe("Service plan schemas", func() {
	It("fetches the plan schema from cloud foundry", func() {
		servicesCurlSession := cf.Cf("curl", fmt.Sprintf("/v2/services?q=label:%s", brokerInfo.ServiceName))
		Expect(servicesCurlSession).To(gexec.Exit(0))
		rawJson := servicesCurlSession.Out.Contents()

		var services CFServices
		err := json.Unmarshal(rawJson, &services)
		Expect(err).NotTo(HaveOccurred(), "could not unmarshal CF services response")
		Expect(len(services.Resources)).To(Equal(1))

		servicePlansURL := services.Resources[0].Entity.ServicePlansURL
		servicePlansCurlSession := cf.Cf("curl", servicePlansURL)
		Expect(servicePlansCurlSession).To(gexec.Exit(0))
		rawJson = servicePlansCurlSession.Out.Contents()

		var servicePlans CFServicePlans
		err = json.Unmarshal(rawJson, &servicePlans)
		Expect(err).NotTo(HaveOccurred(), "could not unmarshal CF service plans response")
		Expect(len(servicePlans.Resources)).To(BeNumerically(">", 0))

		schemas := servicePlans.Resources[0].Entity.Schemas

		for _, schema := range []domain.Schema{schemas.Instance.Create, schemas.Instance.Update, schemas.Binding.Create} {
			createProps, ok := schema.Parameters["properties"]
			Expect(ok).To(BeTrue(), "schema did not contain properties")
			createPropsMap, ok := createProps.(map[string]interface{})
			Expect(ok).To(BeTrue(), "schema properties were not a map")
			Expect(len(createPropsMap)).To(BeNumerically(">", 0), "schema properties were empty")
		}
	})
})
