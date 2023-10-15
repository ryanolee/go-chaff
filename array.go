package chaff

import (
	"fmt"
)

type (
	arrayGenerator struct {
		TupleGenerators          []Generator
		ItemGenerator            Generator
		AdditionalItemsGenerator Generator

		MinItems int
		MaxItems int

		DisallowAdditional bool
		schemaNode 	   schemaNode
	}
)

// Parses the "array" keyword of a schema
// Example:
// {
//   "type": "array",
//   "items": {
//     "type": "string"
//   },
//   "minItems": 1,
//   "maxItems": 10
// }
func parseArray(node schemaNode, metadata *parserMetadata) (Generator, error) {
	// Validate Bounds
	if node.MaxItems != 0 && node.MinItems > node.MaxItems {
		return nullGenerator{}, fmt.Errorf("minItems must be less than or equal to maxItems (minItems: %d, maxItems: %d)", node.MinItems, node.MaxItems)
	}

	if node.MaxContains != 0 && node.MinContains > node.MaxContains {
		return nullGenerator{}, fmt.Errorf("minContains must be less than or equal to maxContains (minContains: %d, maxContains: %d)", node.MinContains, node.MaxContains)
	}

	// Validate if tuple makes sense in this context
	tupleLength := len(node.PrefixItems)
	if tupleLength > node.MaxItems {
		return nullGenerator{}, fmt.Errorf("tuple length must be less than or equal to maxItems (tupleLength: %d, maxItems: %d)", tupleLength, node.MaxItems)
	}

	min := getInt(node.MinItems, node.MinContains)
	max := getInt(node.MaxItems, node.MaxContains)

	// Force the generator to use only the tuple in the event that additional items
	// are not allowed
	if node.Items.DisallowAdditionalItems {
		min = tupleLength
		max = tupleLength
	}

	return arrayGenerator{
		TupleGenerators:          parseTupleGeneratorFromSchemaNode(node, metadata),
		ItemGenerator:            parseItemGenerator(node.Items, metadata),
		AdditionalItemsGenerator: parseAdditionalItems(node, metadata),

		MinItems:           min,
		MaxItems:           max,
		DisallowAdditional: node.Items.DisallowAdditionalItems,
		schemaNode:         node,
	}, nil
}

func parseTupleGeneratorFromSchemaNode(node schemaNode, metadata *parserMetadata) []Generator {
	if len(node.PrefixItems) != 0 {
		return parseTupleGenerator(node.PrefixItems, metadata)
		// Legacy support given "items" when passed as an array
		// has the same meaning as "prefixItems"
	} else if len(node.Items.Nodes) != 0 {
		return parseTupleGenerator(node.Items.Nodes, metadata)
	}
	return nil
}

func parseTupleGenerator(nodes []schemaNode, metadata *parserMetadata) []Generator {
	if len(nodes) == 0 {
		return nil
	}

	generators := []Generator{}
	for i, item := range nodes {
		refPath := fmt.Sprintf("/prefixItems/%d", i)
		generator, err := metadata.ReferenceHandler.ParseNodeInScope(refPath, item, metadata)
		if err != nil {
			generators = append(generators, nullGenerator{})
		} else {
			generators = append(generators, generator)
		}
	}

	return generators
}

func parseAdditionalItems(node schemaNode, metadata *parserMetadata) Generator {
	if node.AdditionalItems == nil {
		return nil
	}

	generator, err := metadata.ReferenceHandler.ParseNodeInScope("/additionalItems", *node.AdditionalItems, metadata)
	if err != nil {
		return nil
	}

	return generator
}

func parseItemGenerator(additionalData itemsData, metadata *parserMetadata) Generator {
	if additionalData.DisallowAdditionalItems || additionalData.Node == nil {
		return nil
	}

	generator, err := metadata.ReferenceHandler.ParseNodeInScope("/items", *additionalData.Node, metadata)
	if err != nil {
		return nil
	}

	return generator
}

func (g arrayGenerator) Generate(opts *GeneratorOptions) interface{} {
	tupleLength := len(g.TupleGenerators)
	arrayData := make([]interface{}, 0)

	if tupleLength != 0 {
		for _, generator := range g.TupleGenerators {
			arrayData = append(arrayData, generator.Generate(opts))
		}
	}

	var itemGen Generator
	itemGen = nullGenerator{}
	if g.ItemGenerator != nil {
		itemGen = g.ItemGenerator
	} else if g.AdditionalItemsGenerator != nil {
		itemGen = g.AdditionalItemsGenerator
	}

	if itemGen != nil || g.DisallowAdditional {
		return arrayData
	}

	minItems := getInt(g.MinItems, opts.DefaultArrayMinItems)
	maxItems := getInt(g.MaxItems, opts.DefaultArrayMaxItems)

	if maxItems < minItems {
		maxItems = minItems + opts.DefaultArrayMaxItems
	}

	remainingItemsToGenerate := maxInt(0, maxItems-tupleLength)

	itemsToGenerate := opts.Rand.RandomInt(0, remainingItemsToGenerate)

	// Generate the remaining items up to a random number
	// (This might skew the distribution of the length of the array)
	for i := 0; i < itemsToGenerate || minItems > len(arrayData); i++ {
		arrayData = append(arrayData, itemGen.Generate(opts))
	}

	return arrayData
}

func (g arrayGenerator) String() string {
	tupleString := ""
	for _, generator := range g.TupleGenerators {
		tupleString += fmt.Sprintf("%s,", generator)
	}

	return fmt.Sprintf("ArrayGenerator{items: %s, tuple: [%s] }", g.ItemGenerator, tupleString)
}
