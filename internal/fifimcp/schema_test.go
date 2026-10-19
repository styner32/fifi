package fifimcp

import (
	"encoding/json"
	"fmt"

	"github.com/fifi/internal/marketreport"
	"github.com/google/jsonschema-go/jsonschema"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func schemaJSON(schema *jsonschema.Schema) map[string]any {
	GinkgoHelper()
	raw, err := json.Marshal(schema)
	Expect(err).NotTo(HaveOccurred())
	var object map[string]any
	Expect(json.Unmarshal(raw, &object)).To(Succeed())
	return object
}

func schemaTypeArrays(value any, path string) []string {
	var found []string
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			childPath := path + "/" + key
			if _, array := child.([]any); key == "type" && array {
				found = append(found, childPath)
			}
			found = append(found, schemaTypeArrays(child, childPath)...)
		}
	case []any:
		for index, child := range value {
			found = append(found, schemaTypeArrays(child, fmt.Sprintf("%s/%d", path, index))...)
		}
	}
	return found
}

func schemaOutputValue(artifacts, warnings any) map[string]any {
	return map[string]any{
		"schema_version": 1, "report_type": "fixture",
		"report":    map[string]any{"missing": nil, "zero": 0, "nested": []any{true, "source", map[string]any{"unconstrained": 1}}},
		"artifacts": artifacts, "warnings": warnings,
	}
}

var _ = Describe("Portable market tool schemas", func() {
	Context("when expressing nullable values", func() {
		It("advertises nullable alternatives without type arrays and leaves other properties and descriptions unchanged", func() {
			output, err := jsonschema.For[marketreport.Envelope](nil)
			Expect(err).NotTo(HaveOccurred())
			input, err := jsonschema.For[pulseArgs](nil)
			Expect(err).NotTo(HaveOccurred())
			for _, fixture := range []struct {
				original, portable *jsonschema.Schema
				properties         []string
				valueType          string
			}{
				{output, marketOutputSchema(), []string{"artifacts", "warnings"}, "array"},
				{input, pulseInputSchema(), []string{"use_history"}, "boolean"},
			} {
				originalJSON, portableJSON := schemaJSON(fixture.original), schemaJSON(fixture.portable)
				for _, name := range fixture.properties {
					original, portable := fixture.original.Properties[name], fixture.portable.Properties[name]
					Expect(portable.Description).To(Equal(original.Description))
					Expect(portable.AnyOf).To(HaveLen(2))
					var alternatives []string
					for _, branch := range portable.AnyOf {
						alternatives = append(alternatives, branch.Type)
					}
					Expect(alternatives).To(ConsistOf("null", fixture.valueType))
					delete(originalJSON["properties"].(map[string]any), name)
					delete(portableJSON["properties"].(map[string]any), name)
				}
				Expect(portableJSON).To(Equal(originalJSON))
				Expect(schemaTypeArrays(schemaJSON(fixture.portable), "")).To(BeEmpty())
			}
		})
	})

	Context("when validating market output", func() {
		var original, portable *jsonschema.Resolved
		BeforeEach(func() {
			schema, err := jsonschema.For[marketreport.Envelope](nil)
			Expect(err).NotTo(HaveOccurred())
			original, err = schema.Resolve(nil)
			Expect(err).NotTo(HaveOccurred())
			portable, err = marketOutputSchema().Resolve(nil)
			Expect(err).NotTo(HaveOccurred())
		})
		DescribeTable("accepts existing nullable and array values without restricting freeform report fields", func(artifacts, warnings any) {
			value := schemaOutputValue(artifacts, warnings)
			Expect(original.Validate(value)).To(Succeed())
			Expect(portable.Validate(value)).To(Succeed())
		},
			Entry("both null", nil, nil),
			Entry("both empty arrays", []any{}, []any{}),
			Entry("null artifacts", nil, []any{"upstream unavailable"}),
			Entry("null warnings", []any{map[string]any{"kind": "json", "path": ".cache/report.json"}}, nil),
			Entry("populated arrays", []any{map[string]any{"kind": "markdown", "path": ".cache/report.md"}}, []any{"partial", "unverified"}),
		)
		DescribeTable("rejects the same malformed array values", func(artifacts, warnings any) {
			value := schemaOutputValue(artifacts, warnings)
			Expect(original.Validate(value)).NotTo(Succeed())
			Expect(portable.Validate(value)).NotTo(Succeed())
		},
			Entry("string artifacts", "[]", []any{}),
			Entry("scalar artifact item", []any{"report.json"}, []any{}),
			Entry("null artifact item", []any{nil}, []any{}),
			Entry("missing artifact path", []any{map[string]any{"kind": "json"}}, []any{}),
			Entry("extra artifact property", []any{map[string]any{"kind": "json", "path": "report.json", "extra": true}}, []any{}),
			Entry("string warnings", []any{}, "warning"),
			Entry("numeric warning item", []any{}, []any{1}),
			Entry("null warning item", []any{}, []any{nil}),
		)
		It("retains exactly the five required output fields and rejects additional root fields", func() {
			Expect(marketOutputSchema().Required).To(ConsistOf("schema_version", "report_type", "report", "artifacts", "warnings"))
			for _, name := range []string{"schema_version", "report_type", "report", "artifacts", "warnings"} {
				value := schemaOutputValue(nil, nil)
				delete(value, name)
				Expect(original.Validate(value)).NotTo(Succeed(), name)
				Expect(portable.Validate(value)).NotTo(Succeed(), name)
			}
			value := schemaOutputValue(nil, nil)
			value["extra"] = true
			Expect(original.Validate(value)).NotTo(Succeed())
			Expect(portable.Validate(value)).NotTo(Succeed())
		})
	})

	Context("when validating pulse input", func() {
		var original, portable *jsonschema.Resolved
		BeforeEach(func() {
			schema, err := jsonschema.For[pulseArgs](nil)
			Expect(err).NotTo(HaveOccurred())
			original, err = schema.Resolve(nil)
			Expect(err).NotTo(HaveOccurred())
			portable, err = pulseInputSchema().Resolve(nil)
			Expect(err).NotTo(HaveOccurred())
		})
		DescribeTable("preserves absent, null and boolean values without injecting a default", func(value map[string]any, expected *bool) {
			Expect(original.Validate(value)).To(Succeed())
			Expect(portable.Validate(value)).To(Succeed())
			initialRaw, err := json.Marshal(value)
			Expect(err).NotTo(HaveOccurred())
			Expect(portable.ApplyDefaults(&value)).To(Succeed())
			raw, err := json.Marshal(value)
			Expect(err).NotTo(HaveOccurred())
			Expect(raw).To(MatchJSON(initialRaw))
			var args pulseArgs
			Expect(json.Unmarshal(raw, &args)).To(Succeed())
			Expect(args.UseHistory).To(Equal(expected))
		},
			Entry("absent", map[string]any{}, nil),
			Entry("null", map[string]any{"use_history": nil}, nil),
			Entry("true", map[string]any{"use_history": true}, jsonschema.Ptr(true)),
			Entry("false", map[string]any{"use_history": false}, jsonschema.Ptr(false)),
		)
		DescribeTable("rejects incompatible types and extra arguments", func(value map[string]any) {
			Expect(original.Validate(value)).NotTo(Succeed())
			Expect(portable.Validate(value)).NotTo(Succeed())
		},
			Entry("string", map[string]any{"use_history": "false"}),
			Entry("number", map[string]any{"use_history": 0}),
			Entry("array", map[string]any{"use_history": []any{false}}),
			Entry("object", map[string]any{"use_history": map[string]any{}}),
			Entry("extra argument", map[string]any{"use_history": true, "command": "arbitrary"}),
		)
	})
})
