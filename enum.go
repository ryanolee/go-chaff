package chaff

import "fmt"

type (
	enumGenerator struct {
		Values []interface{}
	}
)

// Parses the "enum" keyword of a schema
// Example:
//
//	{
//	  "enum": ["foo", "bar"]
//	}
func parseEnum(node schemaNode) (enumGenerator, error) {
	return enumGenerator{
		Values: node.Enum,
	}, nil
}

func (g enumGenerator) Generate(opts *GeneratorOptions) interface{} {
	opts.overallComplexity++
	return opts.Rand.Choice(g.Values)
}

func (g enumGenerator) String() string {
	numberOfItemsInEnum := len(g.Values)
	return fmt.Sprintf("EnumGenerator[items: %d]", numberOfItemsInEnum)
}
