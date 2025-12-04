package chaff

import (
	"fmt"

	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/thoas/go-funk"
)

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
func parseEnum(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if node.Enum == nil || len(*node.Enum) == 0 {
		return nullGenerator{}, fmt.Errorf("enum must be a non-empty array")
	}

	selfSchema, err := metadata.SchemaManager.ParseSchemaNode(metadata, node, "enum")

	if err != nil {
		return nullGenerator{}, fmt.Errorf("failed to compile schema for enum item validation: %w", err)
	}

	validEnumValues, ok := funk.Filter(*node.Enum, func(value interface{}) bool {
		err := selfSchema.Validate(value)
		return err == nil
	}).([]interface{})

	if !ok || len(validEnumValues) == 0 {
		return nullGenerator{}, fmt.Errorf("illogical schema, no enum values match the other schema constraints of the passed node: %v", util.MarshalJsonToString(node.Enum))
	}

	if len(validEnumValues) == 1 {
		return constGenerator{
			Value: validEnumValues[0],
		}, nil
	}

	return enumGenerator{
		Values: validEnumValues,
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
