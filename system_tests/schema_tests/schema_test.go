package schema_tests

import (
	"fmt"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("Service plan schemas", func() {

	It("fetches the plan schema from cloud foundry", func() {
		defaultSchema := `{
  "service_binding": {
	  "create": {
		  "parameters": {
			  "$schema": "http://json-schema.org/draft-04/schema#",
			  "additionalProperties": false,
			  "properties": {
				  "topic": {
					  "description": "The name of the topic",
					  "type": "string"
				  }
			  },
			  "type": "object"
		  }
	  }
  },
  "service_instance": {
	  "create": {
		  "parameters": {
			  "$schema": "http://json-schema.org/draft-04/schema#",
			  "additionalProperties": true,
			  "properties": {
				  "auto_create_topics": {
					  "description": "Auto create topics",
					  "type": "boolean"
				  },
				  "default_replication_factor": {
					  "type": "integer",
					  "description": "Replication factor"
				  }
			  },
			  "type": "object"
		  }
	  },
	  "update": {
		  "parameters": {
			  "$schema": "http://json-schema.org/draft-04/schema#",
			  "additionalProperties": true,
			  "properties": {
				  "auto_create_topics": {
					  "description": "Auto create topics",
					  "type": "boolean"
				  },
				  "default_replication_factor": {
					  "description": "Replication factor",
					  "type": "integer"
				  }
			  },
			  "type": "object"
		  }
	  }
  }
}`

		servicesCurlSession := cf.Cf("curl", fmt.Sprintf("/v2/services?q=label:%s", serviceOffering))
		Eventually(servicesCurlSession).Should(gexec.Exit(0))

		rawJson := servicesCurlSession.Out.Contents()

		entities := getEntities(rawJson)
		Expect(len(entities)).To(Equal(1))

		servicePlansURL, ok := entities[0]["service_plans_url"].(string)
		Expect(ok).To(BeTrue(), "service_plans_url failed to cast as a string")
		Expect(servicePlansURL).NotTo(Equal(""), "Unable to find service_plans_url")

		servicePlansCurlSession := cf.Cf("curl", servicePlansURL)
		Eventually(servicePlansCurlSession).Should(gexec.Exit(0))

		rawServicePlans := servicePlansCurlSession.Out.Contents()
		schemas := getSchemas(rawServicePlans)

		for _, schema := range schemas {
			actualSchema, err := json.Marshal(schema)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualSchema).To(MatchJSON(defaultSchema))
		}
	})
})

func getResources(rawJson []byte) []map[string]interface{} {
	d := make(map[string]interface{})
	json.Unmarshal(rawJson, &d)

	r, ok := d["resources"].([]interface{})
	Expect(ok).To(BeTrue(), "Resources failed to cast to slice")

	resources := make([]map[string]interface{}, len(r))
	for i, v := range r {
		resource, ok := v.(map[string]interface{})
		Expect(ok).To(BeTrue(), fmt.Sprintf("Resource %d failed to cast to map[string]interface{}", i))
		resources[i] = resource
	}

	return resources
}

func getEntities(rawJson []byte) []map[string]interface{} {
	r := getResources(rawJson)

	entities := make([]map[string]interface{}, len(r))
	for i, v := range r {
		entity, ok := v["entity"].(map[string]interface{})
		Expect(ok).To(BeTrue(), fmt.Sprintf("Resource %d's entity failed to cast to map[string]interface{}", i))
		entities[i] = entity
	}

	return entities
}

func getSchemas(rawJson []byte) []map[string]interface{} {
	e := getEntities(rawJson)

	schemas := make([]map[string]interface{}, len(e))
	for i, v := range e {
		schema, ok := v["schemas"].(map[string]interface{})
		Expect(ok).To(BeTrue(), fmt.Sprintf("Entity %d's schema failed to cast to map[string]interface{}", i))
		schemas[i] = schema
	}

	return schemas
}
