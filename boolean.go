package chaff

type (
	booleanGenerator struct {}
)

func parseBoolean(node schemaNode) (booleanGenerator, error) {
	return booleanGenerator{}, nil
}

func (g booleanGenerator) Generate(opts *GeneratorOptions) interface{} {
	return opts.Rand.RandomBool()
}

func (g booleanGenerator) String() string {
	return "BooleanGenerator"
}
