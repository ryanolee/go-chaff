package chaff

import (
	"fmt"
	"strings"

	"github.com/thoas/go-funk"
)

type (
	multipleTypeGenerator struct {
		generators []Generator
	}
)

// Parses the "type" keyword of a schema when it is an array
// Example:
//
//	{
//	  "type": ["string", "number"]
//	}
func parseMultipleType(node schemaNode, metadata *parserMetadata) (multipleTypeGenerator, error) {
	generators := []Generator{}
	for _, nodeType := range node.Type.MultipleTypes {
		generator, err := parseType(nodeType, node, metadata)
		if err != nil {
			generators = append(generators, nullGenerator{})
		} else {
			generators = append(generators, generator)
		}
	}

	return multipleTypeGenerator{
		generators: generators,
	}, nil
}

func (g multipleTypeGenerator) Generate(opts *GeneratorOptions) interface{} {
	generator := g.generators[opts.Rand.RandomInt(0, len(g.generators))]
	return generator.Generate(opts)
}

func (g multipleTypeGenerator) String() string {
	formattedGenerators := funk.Map(g.generators, func(generator Generator) string {
		return generator.String()
	}).([]string)

	return fmt.Sprintf("MultiTypeGenerator{%s}", strings.Join(formattedGenerators, ","))
}
