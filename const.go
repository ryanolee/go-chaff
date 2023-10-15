package chaff

import "fmt"

type (
	ConstGenerator struct {
		Value interface{}
	}
)

// Parses the "const" keyword of a schema
// Example:
// {
//   "const": "foo"
// }
func parseConst(node schemaNode) (ConstGenerator, error) {
	return ConstGenerator{
		Value: node.Const,
	}, nil
}

func (g ConstGenerator) Generate(opts *GeneratorOptions) interface{} {
	return g.Value
}

func (g ConstGenerator) String() string {
	return fmt.Sprintf("ConstGenerator[%s]", g.Value)
}
