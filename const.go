package chaff

import (
	"fmt"

	"github.com/ryanolee/go-chaff/internal/util"
)

type (
	constGenerator struct {
		Value interface{}
	}
)

// Parses the "const" keyword of a schema
// Example:
//
//	{
//	  "const": "foo"
//	}
func parseConst(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if node.Const == nil {
		return nullGenerator{}, fmt.Errorf("const must be defined")
	}

	subSchema, err := metadata.SchemaManager.ParseSchemaNode(metadata, node, "const")
	if err != nil {
		return nullGenerator{}, fmt.Errorf("failed to compile schema for const item validation: %w", err)
	}

	err = subSchema.Validate(*node.Const)
	if err != nil {
		return nullGenerator{}, fmt.Errorf("illogical schema, const value does not match other schema constraints: %v against schema %v with error %s", util.MarshalJsonToString(node.Const), util.MarshalJsonToString(node), err.Error())
	}

	return constGenerator{
		Value: *node.Const,
	}, nil
}

func (g constGenerator) Generate(opts *GeneratorOptions) interface{} {
	return g.Value
}

func (g constGenerator) String() string {
	return fmt.Sprintf("ConstGenerator[%s]", util.MarshalJsonToString(g.Value))
}
