package chaff

type (
	nullGenerator struct {
	}
)

// Parses the "null" type of a schema
// Example:
// {
//   "type": "null"
// }

func parseNull(node schemaNode) (nullGenerator, error) {
	return nullGenerator{}, nil
}

func (g nullGenerator) Generate(opts *GeneratorOptions) interface{} {
	return nil
}

func (g nullGenerator) String() string {
	return "NullGenerator"
}
