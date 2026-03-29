package test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kaptinlin/jsonschema"
	"github.com/ryanolee/go-chaff"
)

// inferredTestFile is the top-level structure of an inferred type test file.
// Each key is a test case name mapping to a pair of schemas.
type inferredTestFile map[string]struct {
	GeneratorSchema json.RawMessage `json:"generator_schema"`
	TestSchema      json.RawMessage `json:"test_schema"`
}

// TestInferredTypeDir runs every .json file in dirPath as an inferred type
// test suite. Each file contains named test cases with a "generator_schema"
// (no explicit type — used for generation) and a "test_schema" (explicit
// type — used for validation). This proves that go-chaff's type inference
// produces values matching the explicitly-typed schema.
func TestInferredTypeDir(t *testing.T, dirPath string, cycles int) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		t.Fatalf("Failed to read directory %s: %s", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		TestInferredTypeFile(t, fmt.Sprintf("%s/%s", dirPath, entry.Name()), cycles)
	}
}

// TestInferredTypeFile runs a single inferred type test file.
func TestInferredTypeFile(t *testing.T, path string, cycles int) {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %s", path, err)
	}

	var suite inferredTestFile
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("Failed to parse %s: %s", path, err)
	}

	for name, tc := range suite {
		t.Run(fmt.Sprintf("InferredType[%s/%s,cycles:%d]", path, name, cycles), func(t *testing.T) {
			// Compile the test schema for validation
			compiler := jsonschema.NewCompiler()
			validationSchema, err := compiler.Compile(tc.TestSchema, name)
			if err != nil {
				t.Fatalf("Failed to compile test_schema: %s", err)
			}

			// Build the generator from the schema without an explicit type
			generator, err := chaff.ParseSchemaWithDefaults(tc.GeneratorSchema)
			if err != nil {
				t.Fatalf("Failed to parse generator_schema: %s", err)
			}

			if generator.Metadata.Errors.HasErrors() {
				for p, genErr := range generator.Metadata.Errors.CollectErrors() {
					t.Logf("generator error [%s]: %s", p, genErr.Error())
				}
				t.Fatalf("Generator schema produced errors")
			}

			for i := 0; i < cycles; i++ {
				value := generator.Generate(&chaff.GeneratorOptions{})
				output, err := json.Marshal(value)
				if err != nil {
					t.Fatalf("Cycle %d: failed to marshal generated value: %s", i, err)
				}

				if res := validationSchema.Validate(output); len(res.Errors) > 0 {
					for field, valErr := range res.Errors {
						t.Logf("- %s: %s", field, valErr.Error())
					}
					t.Fatalf("Cycle %d: generated data from inferred schema did not validate against explicit test_schema:\n%s", i, string(output))
				}
			}
		})
	}
}
