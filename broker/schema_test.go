package broker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

var _ = Describe("Schema validator", func() {
	Context("#ValidateSchema", func() {
		It("does not error when the schema is valid", func() {
			v := broker.NewValidator(exampleSchema)
			err := v.ValidateSchema()

			Expect(err).NotTo(HaveOccurred())
		})

		Context("invalid schema", func() {
			It("errors when the JSON Schema version is missing", func() {
				v := broker.NewValidator(schemaWithMissingSchemaVersion)
				err := v.ValidateSchema()

				Expect(err).To(HaveOccurred())
			})

			It("errors when the JSON Schema version is not draft-04", func() {
				v := broker.NewValidator(schemaWithWrongJSONSchemaVersion)
				err := v.ValidateSchema()

				Expect(err).To(HaveOccurred())
			})

			It("errors when the schema itself fails validation", func() {
				v := broker.NewValidator(invalidSchema)
				err := v.ValidateSchema()

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("#ValidateParams", func() {
		It("panics when the schema has not been validated first", func() {
			params := map[string]interface{}{"hello": "dave"}
			v := broker.NewValidator(exampleSchema)

			validationFunc := func() {
				v.ValidateParams(params)
			}

			Expect(validationFunc).To(Panic())
		})

		It("errors when the provided params are invalid", func() {
			badParams := map[string]interface{}{
				"this-is": "clearly-wrong",
			}

			v := broker.NewValidator(exampleSchema)
			err := v.ValidateSchema()
			Expect(err).NotTo(HaveOccurred())

			err = v.ValidateParams(badParams)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("validation against JSON schema failed - [this-is: Additional property this-is is not allowed]"))
		})

		It("does not error when the params are valid", func() {
			goodParams := map[string]interface{}{
				"default_replication_factor": 5,
				"auto_create_topics":         true,
			}

			v := broker.NewValidator(exampleSchema)
			err := v.ValidateSchema()
			Expect(err).NotTo(HaveOccurred())

			err = v.ValidateParams(goodParams)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var exampleSchema = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-04/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"auto_create_topics": map[string]interface{}{
			"description": "Auto create topics",
			"type":        "boolean",
		},
		"default_replication_factor": map[string]interface{}{
			"description": "Replication factor",
			"type":        "integer",
		},
	},
}

var schemaWithMissingSchemaVersion = map[string]interface{}{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"auto_create_topics": map[string]interface{}{
			"description": "Auto create topics",
			"type":        "boolean",
		},
		"default_replication_factor": map[string]interface{}{
			"description": "Replication factor",
			"type":        "integer",
		},
	},
}

var schemaWithWrongJSONSchemaVersion = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-99/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"auto_create_topics": map[string]interface{}{
			"description": "Auto create topics",
			"type":        "boolean",
		},
		"default_replication_factor": map[string]interface{}{
			"description": "Replication factor",
			"type":        "integer",
		},
	},
}

var invalidSchema = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-04/schema#",
	"type":                 "foo",
	"additionalProperties": false,
	"properties":           map[string]interface{}{},
}
