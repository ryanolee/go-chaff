package chaff

import "fmt"

type (
	constGenerator struct {
		Value interface{}
	}
)

// Parses the "const" keyword of a schema
// Example:
// {
//   "const": "foo"
// }
func parseConst(node schemaNode) (constGenerator, error) {
	return constGenerator{
		Value: node.Const,
	}, nil
}

func (g constGenerator) Generate(opts *GeneratorOptions) interface{} {
	return g.Value
}

func (g constGenerator) String() string {
	return fmt.Sprintf("ConstGenerator[%s]", g.Value)
}
