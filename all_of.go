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

// Parses the "allOf" keyword
func parseAllOf(node schemaNode, metadata *parserMetadata) (Generator, error) {
	mergedNode, err := mergeSchemaNodes(metadata, node.AllOf...)
	if err != nil {
		return &NullGenerator{}, err
	}

	generator, err := parseSchemaNode(mergedNode, metadata)
	if err != nil {
		return &NullGenerator{}, err
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
