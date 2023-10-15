package chaff

import "fmt"

type (
	EnumGenerator struct {
		Values []interface{}
	}
)

// Parses the "enum" keyword of a schema
// Example:
// {
//   "enum": ["foo", "bar"]
// }
func parseEnum(node schemaNode) (EnumGenerator, error) {
	return EnumGenerator{
		Values: node.Enum,
	}, nil
}

func (g EnumGenerator) Generate(opts *GeneratorOptions) interface{} {
	return opts.Rand.Choice(g.Values)
}

func (g EnumGenerator) String() string {
	numberOfItemsInEnum := len(g.Values)
	return fmt.Sprintf("EnumGenerator[items: %d]", numberOfItemsInEnum)
}
