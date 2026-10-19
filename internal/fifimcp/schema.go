package fifimcp

import (
	"github.com/fifi/internal/marketreport"
	"github.com/google/jsonschema-go/jsonschema"
)

func marketOutputSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[marketreport.Envelope](nil)
	if err != nil {
		panic(err) // Static Go types must produce valid tool schemas.
	}
	for _, name := range []string{"artifacts", "warnings"} {
		schema.Properties[name] = nullableAnyOf(schema.Properties[name], "array")
	}
	return schema
}

func pulseInputSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[pulseArgs](nil)
	if err != nil {
		panic(err)
	}
	schema.Properties["use_history"] = nullableAnyOf(schema.Properties["use_history"], "boolean")
	return schema
}

// Some MCP clients reject JSON Schema's type arrays. Keep the inferred value
// constraints intact while spelling the same nullable value as two alternatives.
func nullableAnyOf(property *jsonschema.Schema, valueType string) *jsonschema.Schema {
	value := *property
	value.Type, value.Types, value.Description = valueType, nil, ""
	return &jsonschema.Schema{
		Description: property.Description,
		AnyOf:       []*jsonschema.Schema{{Type: "null"}, &value},
	}
}
