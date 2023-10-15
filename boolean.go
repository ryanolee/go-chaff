package chaff

type (
	booleanGenerator struct {}
)

// Parses the "boolean" keyword of a schema
// Example:
// {
//   "type": "boolean"
// }
func parseBoolean(node schemaNode) (booleanGenerator, error) {
	return booleanGenerator{}, nil
}

func (g booleanGenerator) Generate(opts *GeneratorOptions) interface{} {
	return opts.Rand.RandomBool()
}

func (g booleanGenerator) String() string {
	return "BooleanGenerator"
}
