package chaff

import (
	"fmt"
)

// Generator for the "allOf" keyword
type allOfGenerator struct {
	SchemaNodes []schemaNode
	MergedNode  schemaNode
	Generator   Generator
}

// Parses the "allOf" keyword. This generator is experimental and may not work as expected.
// Known issues:
//  - Reference resolution is not supported
//  - The merging algorithm does not 100 percent align with the way
//    the spec expects things to work
//  - It will not throw an error if the merged schema is invalid or illogical
// Example:
// {
//   "allOf": [
//     { "type": "string" },
//     { "format": "ipv4" }
//   ]
// }
func parseAllOf(node schemaNode, metadata *parserMetadata) (Generator, error) {
	mergedNode, err := mergeSchemaNodes(metadata, node.AllOf...)
	if err != nil {
		return &nullGenerator{}, err
	}

	generator, err := parseSchemaNode(mergedNode, metadata)
	if err != nil {
		return &nullGenerator{}, err
	}

	return &allOfGenerator{
		SchemaNodes: node.AllOf,
		MergedNode:  mergedNode,
		Generator:   generator,
	}, nil
}

func (g *allOfGenerator) Generate(opts *GeneratorOptions) interface{} {
	return g.Generator.Generate(opts)
}

func (g *allOfGenerator) String() string {
	return fmt.Sprintf("AllOfGenerator[%s]", g.Generator)
}
