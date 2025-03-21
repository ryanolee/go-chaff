package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/ryanolee/go-chaff"
	"github.com/ryanolee/go-chaff/rand"
	"github.com/santhosh-tekuri/jsonschema"
)

func TestJsonSchemaDir(test *testing.T, dirPath string, cycles int) {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		test.Fatalf("Failed to read directory: %s", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		TestJsonSchema(test, fmt.Sprintf("%s/%s", dirPath, file.Name()), cycles)
	}
}

func TestJsonSchema(test *testing.T, schemaPath string, cycles int) {
	test.Run(fmt.Sprintf("GenerativeTest[%s,cycles:%d]", schemaPath, cycles), func(test *testing.T) {
		if cycles < 1 {
			cycles = 100
		}

		schema, err := jsonschema.Compile(schemaPath)
		if err != nil {
			test.Fatalf("Failed to compile schema: %s", err)
		}

		generator, err := chaff.ParseSchemaFile(schemaPath, &chaff.ParserOptions{})
		if err != nil {
			test.Fatalf("Failed to compile generator: %s", err)
		}

		if len(generator.Metadata.Errors) > 0 {
			test.Fatalf("Failed to compile generator: %s", generator.Metadata.Errors)
		}

		for i := 0; i < cycles; i++ {
			data, err := json.MarshalIndent(generator.Generate(&chaff.GeneratorOptions{
				Rand:                       rand.NewRandUtilFromTime(),
				BypassCyclicReferenceCheck: true,
			}), "", "    ")

			if err != nil {
				test.Fatalf("Failed to serialize JSON: %s", err)
			}

			if err := schema.Validate(strings.NewReader(string(data))); err != nil {
				test.Fatalf("Failed to validate instance: %s. Data: %s", err, data)
			}
		}
	})
}
