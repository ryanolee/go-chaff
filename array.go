package chaff

import (
	"fmt"

	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/thoas/go-funk"
)

type (
	arrayGenerator struct {
		TupleGenerators           []Generator
		ItemGenerator             Generator
		AdditionalItemsGenerator  Generator
		ContainsGenerator         Generator
		UnevaluatedItemsGenerator Generator
		DisallowUnevaluatedItems  bool

		UniqueItems bool

		MinContains int
		MinItems    int
		MaxItems    int

		DisallowAdditional bool
		schemaNode         schemaNode
	}
)

// Parses the "array" keyword of a schema
// Example:
//
//	{
//	  "type": "array",
//	  "items": {
//	    "type": "string"
//	  },
//	  "minItems": 1,
//	  "maxItems": 10
//	}
func parseArray(node schemaNode, metadata *parserMetadata) (Generator, error) {
	// Handle case where contains is set with no minContains (at least 1 must match subschema)
	minContains := util.GetZeroIfNil(node.MinContains, 0)
	maxContains := util.GetZeroIfNil(node.MaxContains, 0)
	minItems := util.GetZeroIfNil(node.MinItems, 0)
	maxItems := util.GetZeroIfNil(node.MaxItems, 0)
	prefixItems := util.GetZeroIfNil(node.PrefixItems, []schemaNode{})

	if minContains == 0 && node.Contains != nil {
		minContains = 1
	}

	// Validate Bounds
	if err := assertLowerUpperBound(minItems, maxItems, "minItems", "maxItems"); err != nil {
		return nullGenerator{}, err
	}

	if err := assertLowerUpperBound(minContains, maxContains, "minContains", "maxContains"); err != nil {
		return nullGenerator{}, err
	}

	if err := assertLowerUpperBound(minContains, minItems, "minContains", "minItems"); err != nil {
		return nullGenerator{}, err
	}

	if err := assertLowerUpperBound(len(prefixItems)+minContains, maxItems, "Tuple items Plus Prefix items. (Note contains does not assume tuple items count towards the total account)", "minItems"); err != nil {
		return nullGenerator{}, err
	}

	// Validate if tuple makes sense in this context
	tupleLength := len(prefixItems)
	if tupleLength > maxItems && node.MaxItems != nil {
		return nullGenerator{}, fmt.Errorf("tuple length must be less than or equal to maxItems (tupleLength: %d, maxItems: %d)", tupleLength, node.MaxItems)
	}

	min := util.GetInt(minItems, minContains)
	max := util.GetInt(maxItems, min+defaultOffset)

	disallowedAdditionalItems := (node.Items != nil && node.Items.DisallowAdditionalItems) || (node.AdditionalItems != nil && node.AdditionalItems.IsFalse)

	// Force the generator to use only the tuple in the event that additional items
	// are not allowed
	if disallowedAdditionalItems {
		min = tupleLength
		max = tupleLength
	}

	var itemGenerator, additionalItemGenerator, containsGenerator, unevaluatedItemsGenerator Generator = nil, nil, nil, nil
	var tupleGenerators []Generator = nil
	var err error = nil

	// Parse sub-generators
	if itemGenerator, err = parseItemGenerator(node.Items, metadata); err != nil {
		return nullGenerator{}, fmt.Errorf("error parsing item generator: %w", err)
	}

	if additionalItemGenerator, err = parseAdditionalItems(node, metadata); err != nil {
		return nullGenerator{}, fmt.Errorf("error parsing additional item generator: %w", err)
	}

	if containsGenerator, err = parseContainsGeneratorInScope(node, metadata); err != nil {
		return nullGenerator{}, fmt.Errorf("error parsing contains generator: %w", err)
	}

	if tupleGenerators, err = parseTupleGeneratorFromSchemaNode(node, metadata); err != nil {
		return nullGenerator{}, fmt.Errorf("error parsing tuple generator: %w", err)
	}

	disallowUnevaluatedItems := node.UnevaluatedItems != nil && node.UnevaluatedItems.IsFalse

	if !disallowUnevaluatedItems && node.UnevaluatedItems != nil && node.UnevaluatedItems.Schema != nil {
		if unevaluatedItemsGenerator, err = metadata.ReferenceHandler.ParseNodeInScope("/unevaluatedItems", *node.UnevaluatedItems.Schema, metadata); err != nil {
			return nullGenerator{}, fmt.Errorf("error parsing unevaluated items generator: %w", err)
		}
	}

	return arrayGenerator{
		TupleGenerators: tupleGenerators,
		ItemGenerator:   itemGenerator,

		UnevaluatedItemsGenerator: unevaluatedItemsGenerator,
		DisallowUnevaluatedItems:  disallowUnevaluatedItems,

		DisallowAdditional:       node.Items != nil && node.Items.DisallowAdditionalItems,
		AdditionalItemsGenerator: additionalItemGenerator,

		MinContains:       minContains,
		ContainsGenerator: containsGenerator,

		MinItems: min,
		MaxItems: max,

		UniqueItems: util.GetZeroIfNil(node.UniqueItems, false),

		schemaNode: node,
	}, nil
}

func parseTupleGeneratorFromSchemaNode(node schemaNode, metadata *parserMetadata) ([]Generator, error) {

	if node.PrefixItems != nil && len(*node.PrefixItems) != 0 {
		return parseTupleGenerator(*node.PrefixItems, metadata)
		// Legacy support given "items" when passed as an array
		// has the same meaning as "prefixItems"
	} else if node.Items != nil && node.Items.Nodes != nil && len(*node.Items.Nodes) != 0 {
		return parseTupleGenerator(*node.Items.Nodes, metadata)
	}
	return nil, nil
}

func parseTupleGenerator(nodes []schemaNode, metadata *parserMetadata) ([]Generator, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	generators := []Generator{}
	for i, item := range nodes {
		refPath := fmt.Sprintf("/prefixItems/%d", i)
		generator, err := metadata.ReferenceHandler.ParseNodeInScope(refPath, item, metadata)
		if err != nil {
			err = fmt.Errorf("error parsing tuple generator: %w", err)
			return nil, err
		}

		generators = append(generators, generator)
	}

	return generators, nil
}

func parseAdditionalItems(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if node.AdditionalItems == nil || node.AdditionalItems.Schema == nil {
		return nil, nil
	}

	return metadata.ReferenceHandler.ParseNodeInScope("/additionalItems", *node.AdditionalItems.Schema, metadata)

}

func parseItemGenerator(additionalData *itemsData, metadata *parserMetadata) (Generator, error) {
	if additionalData == nil {
		return nil, nil
	}

	if additionalData.DisallowAdditionalItems || additionalData.Node == nil {
		return nil, nil
	}

	return metadata.ReferenceHandler.ParseNodeInScope("/items", *additionalData.Node, metadata)
}

func parseContainsGeneratorInScope(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if node.Contains == nil {
		return nil, nil
	}

	var err error = nil
	mergedContains := *node.Contains
	// Merge with items if possible to constrain the contains generator further to aling with item constraints
	if node.Items != nil && node.Items.Node != nil {
		mergedContains, err = mergeSchemaNodes(metadata, *node.Items.Node, *node.Contains)
		if err != nil {
			return nil, fmt.Errorf("error merging 'contains' with 'items' schema: %w", err)
		}
		mergedContains.Contains = nil
	}

	return metadata.ReferenceHandler.ParseNodeInScope("/contains", mergedContains, metadata)
}

func (g arrayGenerator) Generate(opts *GeneratorOptions) interface{} {
	opts.overallComplexity++
	if opts.ShouldCutoff() {
		return nil
	}

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
	} else if g.UnevaluatedItemsGenerator != nil {
		// Fallback to unevaluated items if no other generator is set
		itemGen = g.UnevaluatedItemsGenerator
	} else if g.UniqueItems {
		// Use higher entropy generator if unique items are required
		itemGen = stringGenerator{}
	}

	if g.DisallowAdditional {
		return arrayData
	}

	minItems := util.GetInt(g.MinItems, opts.DefaultArrayMinItems)
	maxItems := util.GetInt(g.MaxItems, opts.DefaultArrayMaxItems)

	// Handle cases where no min items are handled
	minContains := 0
	if g.ContainsGenerator != nil {
		minContains = util.GetInt(g.MinContains, 1)
	}

	// Generate any required "contains" items
	for i := 0; i < minContains; i++ {
		item, ok := g.generateConsideringUnique(opts, g.ContainsGenerator, arrayData)
		if !ok && i > minContains {
			// If we are unable to generate a unique item, and we have already satisfied the minimum contains
			// bail out early
			break
		}

		arrayData = append(arrayData, item)
	}

	if maxItems < minItems {
		maxItems = minItems + opts.DefaultArrayMaxItems
	}

	// Compute how many items we can generate over the minimum satisfiable set of data)
	remainingItemsToGenerate := util.MaxInt(0, maxItems-(tupleLength+g.MinContains))

	itemsToGenerate := opts.Rand.RandomInt(0, remainingItemsToGenerate)

	// Cull the remaining items if the complexity is too high or unevaluated items are not allowed
	if g.DisallowUnevaluatedItems || opts.ShouldMinimize() {
		itemsToGenerate = 0
	}

	// Generate the remaining items up to a random number
	// (This might skew the distribution of the length of the array)
	for i := 0; i < itemsToGenerate || minItems > len(arrayData); i++ {
		item, ok := g.generateConsideringUnique(opts, itemGen, arrayData)

		if !ok && i > minItems {
			// If we are unable to generate a unique item, and we have already satisfied the minimum
			// bail out early
			break
		}

		arrayData = append(arrayData, item)
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

// Will attempt to generate a unique item if the uniqueItems flag is set
func (g arrayGenerator) generateConsideringUnique(opts *GeneratorOptions, itemGenerator Generator, arrayData []interface{}) (interface{}, bool) {
	if !g.UniqueItems {
		return itemGenerator.Generate(opts), true
	}

	currentItems := funk.Map(arrayData, util.MarshalJsonToString).([]string)

	// Generate until we have a unique item
	for i := 0; i < opts.MaximumUniqueGeneratorAttempts; i++ {
		item := itemGenerator.Generate(opts)
		if !funk.Contains(currentItems, util.MarshalJsonToString(item)) {
			return item, true
		}
	}

	return fmt.Sprintf("Warning: Unable to generate unique item after %d attempts. Recheck passed schema.", opts.MaximumUniqueGeneratorAttempts), false
}
