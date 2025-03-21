package chaff

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thoas/go-funk"
)

type (
	combinationGenerator struct {
		Generators []Generator
		Type       string
	}
)

// Parses the "oneOf" or "anyOf" keyword of a schema. This generator is experimental and may not work as expected.
// Example:
//
//	{
//	  "oneOf": [
//	    { "type": "string" },
//	    { "type": "number" }
//	  ]
//	}
//
// One of has a similar implementation to anyOf, so they are both handled by this function
// There are some edge cases that are not handled by this function, such as:
//   - During "factoring" of the schema merging might not work as expected (Reference resolution is not supported as part of this)
//   - oneOf Does not actually validate that only one of the schemas is valid.
func parseCombination(node schemaNode, metadata *parserMetadata) (Generator, error) {
	ref := metadata.ReferenceHandler
	if len(node.OneOf) == 0 && len(node.AnyOf) == 0 {
		return nullGenerator{}, errors.New("no items specified for oneOf / anyOf")
	}

	if len(node.OneOf) > 0 && len(node.AnyOf) > 0 {
		return nullGenerator{}, errors.New("only one of [oneOf / anyOf] can be specified")
	}

	target := node.OneOf
	nodeType := "oneOf"
	if len(node.AnyOf) > 0 {
		target = node.AnyOf
		nodeType = "anyOf"
	}

	generators := []Generator{}
	for i, subSchema := range target {
		baseNode, _ := mergeSchemaNodes(metadata, node)
		baseNode.OneOf = nil
		baseNode.AnyOf = nil

		mergedNode, err := mergeSchemaNodes(metadata, baseNode, subSchema)
		if err != nil {
			generators = append(generators, nullGenerator{})
			continue
		}

		refPath := fmt.Sprintf("/%s/%d", nodeType, i)
		generator, err := ref.ParseNodeInScope(refPath, mergedNode, metadata)
		if err != nil {
			generators = append(generators, nullGenerator{})
		} else {
			generators = append(generators, generator)
		}
	}

	return combinationGenerator{
		Generators: generators,
		Type:       nodeType,
	}, nil
}

func (g combinationGenerator) Generate(opts *GeneratorOptions) interface{} {
	// Select a random generator
	generator := g.Generators[opts.Rand.RandomInt(0, len(g.Generators))]
	return generator.Generate(opts)
}

func (g combinationGenerator) String() string {
	formattedGenerators := funk.Map(g.Generators, func(generator Generator) string {
		return generator.String()
	}).([]string)
	return fmt.Sprintf("CombinationGenerator[%s]{%s}", g.Type, strings.Join(formattedGenerators, ","))
}
