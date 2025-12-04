package main

import (
	"fmt"
	"log"

	"github.com/kaptinlin/jsonschema"
)

func main() {
	schemaJSON := `{
		"type": "string",
		"minLength": 1,
		"not": {
			"format": "uuid"
		}
	}`

	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile([]byte(schemaJSON))
	if err != nil {
		log.Fatalf("Failed to compile schema: %v", err)
	}

	testCases := []struct {
		name     string
		value    string
		expected bool // true = should be valid, false = should be invalid
	}{
		// Valid cases - strings that are NOT UUIDs
		{"regular string", "hello world", true},
		{"number string", "12345", true},
		{"partial uuid", "550e8400-e29b", true},

		// Invalid cases - valid UUIDs should fail validation
		{"valid uuid lowercase", "550e8400-e29b-41d4-a716-446655440000", false},
		{"valid uuid uppercase", "550E8400-E29B-41D4-A716-446655440000", false},
		{"valid uuid mixed case", "550e8400-E29B-41d4-A716-446655440000", false},
	}

	fmt.Println("Testing 'not' with format: uuid")
	fmt.Println("================================")

	for _, tc := range testCases {
		result := schema.Validate(tc.value)
		isValid := result.IsValid()

		status := "✓ PASS"
		if isValid != tc.expected {
			status = "✗ FAIL (FALSE POSITIVE)"
		}

		fmt.Printf("%s: value=%q, valid=%v, expected=%v, %s\n",
			tc.name, tc.value, isValid, tc.expected, status)

		if !result.IsValid() {
			for _, d := range result.Errors {
				fmt.Printf("  Error: %s\n", d.Error())
			}
		}
	}
}
