package chaff

import (
	"fmt"
)

type (
	// Root generator a given schema. Call the Generate method on this to generate a value
	rootGenerator struct {
		Generator Generator
		// For any "$defs"
		Defs map[string]Generator
		// For any "definitions"
		Definitions map[string]Generator
		// Metadata related to parser operations
		Metadata    *parserMetadata
	}
)

// Parses the top-level properties of a schema (including "$defs" and "definitions")
// Example:
// {
//   "type": "object",
//   "$defs": {
//     "foo": {
//       "type": "string"
//     }
//   },
//   "properties": {
//     "bar": {
//       "$ref": "#/$defs/foo"
//     }
//   }
// }
func parseRoot(node schemaNode, metadata *parserMetadata) (rootGenerator, error) {
	def := parseDefinitions("$defs", metadata, node.Defs)
	definitions := parseDefinitions("definitions", metadata, node.Definitions)

	generator, err := parseNode(node, metadata)
	return rootGenerator{
		Generator:   generator,
		Defs:        def,
		Definitions: definitions,
		Metadata:    metadata,
	}, err
}

func parseDefinitions(path string, metadata *parserMetadata, definitions map[string]schemaNode) map[string]Generator {
	ref := metadata.ReferenceHandler
	generators := make(map[string]Generator)
	for key, value := range definitions {
		refPath := fmt.Sprintf("/%s/%s", path, key)
		generator, _ := ref.ParseNodeInScope(refPath, value, metadata)

		generators[key] = generator
	}

	return generators
}

// Generates values based on the passed options
func (g rootGenerator) Generate(opts *GeneratorOptions) interface{} {
	opts = withGeneratorOptionsDefaults(*opts)
	return g.Generator.Generate(opts)
}

func (g rootGenerator) GenerateWithDefaults() interface{} {
	opts := withGeneratorOptionsDefaults(GeneratorOptions{})
	return g.Generator.Generate(opts)
}

func (g rootGenerator) String() string {
	formattedString := ""
	for name, prop := range g.Definitions {
		formattedString += fmt.Sprintf("%s: %s,", name, prop)
	}

	formattedString += "$defs:"
	for name, prop := range g.Defs {
		formattedString += fmt.Sprintf("%s: %s,", name, prop)
	}

	return fmt.Sprintf("RootGenerator{Generator: %s Definitions: %s}", g.Generator, formattedString)
}
