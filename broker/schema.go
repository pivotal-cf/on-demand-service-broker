package broker

import (
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

type Validator struct {
	rawSchema map[string]interface{}
	schema    gojsonschema.JSONLoader
}

func NewValidator(rawSchema map[string]interface{}) *Validator {
	return &Validator{
		rawSchema: rawSchema,
		schema:    nil,
	}
}

func (v *Validator) ValidateParams(params map[string]interface{}) (bool, error) {
	if v.schema == nil {
		panic("ValidateSchema() must be called before ValidateParams()")
	}

	paramsLoader := gojsonschema.NewGoLoader(params)

	result, err := gojsonschema.Validate(v.schema, paramsLoader)
	if err != nil {
		return false, fmt.Errorf("error occurred when attempting to validate against JSON schema")
	}

	return result.Valid(), nil
}

func (v *Validator) ValidateSchema() error {
	version, ok := v.rawSchema["$schema"]
	if !ok {
		return fmt.Errorf("failed validating schema - no JSON Schema provided")
	}

	versionStr, ok := version.(string)
	if !ok || versionStr != "http://json-schema.org/draft-04/schema#" {
		return fmt.Errorf("failed validating schema - only JSON Schema version draft-04 is supported")
	}

	loader := gojsonschema.NewGoLoader(v.rawSchema)
	_, err := gojsonschema.NewSchema(loader)
	if err != nil {
		return fmt.Errorf("failed validating schema - schema does not conform to JSON Schema spec")
	}

	v.schema = loader

	return nil
}
