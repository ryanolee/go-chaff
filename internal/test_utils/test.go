package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/kaptinlin/jsonschema"
	"github.com/ryanolee/go-chaff"
)

func TestJsonSchemaDir(test *testing.T, dirPath string, cycles int) {
	TestJsonSchemaDirWithConfig(test, dirPath, cycles, nil, nil)
}

func TestJsonSchemaDirWithConfig(test *testing.T, dirPath string, cycles int, options *chaff.ParserOptions, getGeneratorOptions func() *chaff.GeneratorOptions) {
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

		TestJsonSchema(test, fmt.Sprintf("%s/%s", dirPath, file.Name()), cycles, options, getGeneratorOptions)
	}
}

func TestJsonSchema(test *testing.T, schemaPath string, cycles int, options *chaff.ParserOptions, getGeneratorOptions func() *chaff.GeneratorOptions) {
	if getGeneratorOptions == nil {
		getGeneratorOptions = func() *chaff.GeneratorOptions {
			return &chaff.GeneratorOptions{
				BypassCyclicReferenceCheck: false,
			}
		}
	}
	test.Run(fmt.Sprintf("GenerativeTest[%s,cycles:%d]", schemaPath, cycles), func(test *testing.T) {
		if cycles < 1 {
			cycles = 100
		}

		// Read and compile schema
		fileData, err := ioutil.ReadFile(schemaPath)
		if err != nil {
			test.Fatalf("Failed to read schema file: %s", err)
		}

		compiler := jsonschema.NewCompiler()
		schema, err := compiler.Compile(fileData, schemaPath)

		if err != nil {
			test.Fatalf("Failed to compile schema: %s", err)
		}

		if options == nil {
			options = &chaff.ParserOptions{}
		}

		generator, err := chaff.ParseSchemaFile(schemaPath, options)
		if err != nil {
			test.Fatalf("Failed to compile generator: %s", err)
		}

		if generator.Metadata.Errors.HasErrors() {
			for path, genErr := range generator.Metadata.Errors.CollectErrors() {
				test.Logf("\n===============ERROR [%s]============\n%s\n=======================END ERROR=================\n\n\n", path, genErr.Error())
			}
			test.Fatalf("Failed to compile generator due to above errors")
		}

		for i := 0; i < cycles; i++ {
			data, err := json.MarshalIndent(generator.Generate(
				getGeneratorOptions(),
			), "", "    ")

			if err != nil {
				test.Fatalf("Failed to serialize JSON: %s", err)
			}

			if res := schema.Validate(data); len(res.Errors) > 0 {
				for field, err := range res.Errors {
					test.Logf("- %s: %s\n", field, err.Error())
				}
				test.Fatalf("Generated data did not validate against schema on cycle %d:\n%s", i, string(data))

			}
		}
	})
}
