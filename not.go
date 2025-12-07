package chaff

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/thoas/go-funk"
)

type (
	notMergeFunction func(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error
)

// N.b Order matters here
var notMergeFunctions []notMergeFunction

func init() {
	notMergeFunctions = []notMergeFunction{
		notApplyType,
		notApplyString,
		notApplyNumberAndInteger,
		notApplyEnum,
		notApplyConst,
		notApplyArray,
		notApplyObject,
		applyUnsupportedNotFields,
	}
}

// Parses the "not" type of a schema
// Example:
// {
//   "not": {"type": "null"}
// }

// Strategy:
//   Recursively coerces the node tree to account for the "not" node where possible
//   and applying constraints post the value being generated if not
//   then hands off the node for regular parsing based on the new node

func parseNot(node schemaNode, metadata *parserMetadata) (Generator, error) {
	// Flatten the not node structure
	metadata.ReferenceHandler.PushToPath("/not")
	notNode, err := mergeSchemaNodes(metadata, schemaNode{}, *node.Not)
	metadata.ReferenceHandler.PopFromPath("/not")

	if err != nil {
		return nullGenerator{}, err
	}

	// Flatten the existing structure
	flatNode, err := mergeSchemaNodes(metadata, node)
	flatNode.Not = nil
	if err != nil {
		return nullGenerator{}, err
	}

	// Cascade merge the flattened nodes & Create a generator from said nodes
	newNode, constraints := notMerge(metadata, flatNode, notNode)
	internalGenerator, err := parseNode(newNode, metadata)

	if err != nil {
		return nullGenerator{}, err
	}

	return constrainedGenerator{
		internalGenerator: internalGenerator,
		constraints: []constraint{
			constraints.Compile(),
		},
	}, nil
}

// Handles cases where double negation occurs in not nodes (e.g. not/not)
// or triple negation (not/not/not) etc where every second order not is merged into its parent
func handleNotSimplification(node schemaNode, metadata *parserMetadata) (schemaNode, error) {
	currentNode := node

	// Separate not nodes from double inverted nodes
	notNodes := []schemaNode{}
	doubleInvertedNodes := []schemaNode{}
	for currentNode.Not != nil {
		notNodes = append(notNodes, *node.Not)
		if currentNode.Not.Not == nil {
			break
		}

		doubleInvertedNodes = append(doubleInvertedNodes, *currentNode.Not.Not)
		currentNode = *currentNode.Not.Not
	}

	// If we have no double inverted nodes then nothing to do and we can leave it early
	if len(doubleInvertedNodes) == 0 {
		return node, nil
	}

	// Begin merging double inverted nodes back into the parent nodes
	for i, doubleInvertedNode := range doubleInvertedNodes {
		node.Not = nil
		path := strings.Repeat("/not/not", i+1)
		metadata.ReferenceHandler.PushToPath(path)
		var err error
		node, err = mergeSchemaNodes(metadata, node, doubleInvertedNode)
		metadata.ReferenceHandler.PopFromPath(path)
		if err != nil {
			warnField(metadata, path, fmt.Errorf("failed to simplify double negation: %w", err))
			continue
		}
	}

	// Rebuild the not node from the remaining not nodes that were not double inverted
	notNode := schemaNode{}
	metadata.ReferenceHandler.PushToPath("/not")
	for i, notNode := range notNodes {
		notNode.Not = nil
		path := strings.Repeat("/not/not", i)
		metadata.ReferenceHandler.PushToPath(path)
		var err error
		notNode, err = mergeSchemaNodes(metadata, notNode, notNode)
		metadata.ReferenceHandler.PopFromPath(path)
		if err != nil {
			warnField(metadata, path, fmt.Errorf("failed to simplify negation: %w", err))
			continue
		}
	}
	metadata.ReferenceHandler.PopFromPath("/not")

	// Serialize to check if not node is empty
	notNodeAsJson, err := json.Marshal(notNode)
	if err != nil {
		return node, err
	}

	if string(notNodeAsJson) == "{}" {
		return node, nil
	}

	node.Not = &notNode

	return node, nil
}

// Merges two nodes, one with candidate
func notMerge(metadata *parserMetadata, node schemaNode, notNode schemaNode) (schemaNode, *constraintCollection) {
	var newNode schemaNode
	constraints := newConstraintCollection()

	for _, notApplyFunc := range notMergeFunctions {
		if err := notApplyFunc(metadata, &newNode, &constraints, node, notNode); err != nil {
			return newNode, &constraints
		}

	}

	newNode.Not = nil

	return newNode, &constraints

	// Note unsupported patterns (Regexes and formats are to complex to "not" generate)

	// Handle bounds
	// - Min/Max Items
	// - Min/Max Contains
	// - Min/Max Properties

	//
	//newNode.MinProperties, newNode.MaxProperties = resolveBoundsInt(
	//	metadata,
	//	"minProperties", "maxProperties",
	//	node.MinProperties, node.MaxProperties,
	//	notNode.MinProperties, notNode.MaxProperties,
	//)

	// Make sure to disaccociate the exiting node

}

// Handles not case for
//   - 'type'
func notApplyType(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error {
	// Collect type
	newNodeType, err := resolveType(node, notNode)
	// Print types from node types
	if err != nil {
		metadata.Errors.AddError(err)
		return err
	}

	newNode.Type = &newNodeType
	return nil
}

// Handles not case for string properties
// - pattern (Unsupported: Other details need confirming)
// - regex (Unsupported: Impossible to generate accurately for)
// - minLength
// - maxLength
func notApplyString(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error {
	//Additional fields
	newNode.MinLength, newNode.MaxLength = resolveBoundsInt(
		metadata,
		"minLength", "maxLength",
		node.MinLength, node.MaxLength,
		notNode.MinLength, notNode.MaxLength,
	)

	newNode.Pattern = node.Pattern
	newNode.Format = node.Format

	if notNode.Pattern != nil {
		if util.GetZeroIfNil(newNode.Pattern, "") == *notNode.Pattern {
			warnField(metadata, "not/pattern", fmt.Errorf("cannot have 'pattern' and 'not/pattern' set to the same value, they are mutually exclusive"))
			return nil
		}

		if err := constraintCollection.AddNotMatchingRegexConstraint(*notNode.Pattern); err != nil {
			warnField(metadata, "not/pattern", fmt.Errorf("invalid regex pattern given in not clause: %s", *notNode.Pattern))
			return nil
		}

		return nil
	}

	if notNode.Format != nil {
		if util.GetZeroIfNil(newNode.Format, "") == *notNode.Format {
			warnField(metadata, "not/format", fmt.Errorf("cannot have 'format' and 'not/format' set to the same value, they are mutually exclusive"))
			return nil
		}

		if err := constraintCollection.AddNotMatchingFormatConstraint(*notNode.Format); err != nil {
			warnField(metadata, "not/format", fmt.Errorf("invalid format given in not clause: %s", *notNode.Format))
			return nil
		}
	}

	return nil
}

// Handles merging logic for 'integer' and 'numbers' using the following fields
//   - multipleOf
//   - minimum
//   - maximum
//   - exclusiveMinimum
//   - exclusiveMaximum
func notApplyNumberAndInteger(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error {

	// Handle integer verses float64 offsets
	offsetIncrement := infinitesimal
	if newNode.Type != nil && nodeTypeContains(newNode, string(generatorTypeInteger)) {
		offsetIncrement = 1
	}

	// Handle min/max
	min, max, alturnateMin, alturnateMax := resolveBoundsFloat64(
		metadata,
		"minimum", "maximum",
		node.Minimum, node.ExclusiveMinimum,
		node.Maximum, node.ExclusiveMaximum,
		notNode.Minimum, notNode.ExclusiveMinimum,
		notNode.Maximum, notNode.ExclusiveMaximum,
		offsetIncrement,
	)

	newNode.Minimum = min
	newNode.Maximum = max

	// Handle multipleOf
	multipleOf := util.GetZeroIfNil(node.MultipleOf, 0)
	notMultipleOf := util.GetZeroIfNil(notNode.MultipleOf, 0)

	couldBeInteger := nodeTypeContains(newNode, string(generatorTypeInteger))
	explicitDenyInteger := nodeTypeContains(&notNode, string(generatorTypeInteger))

	if multipleOf == 0 && couldBeInteger && !explicitDenyInteger {
		multipleOf = 1
	}

	// Avoid multiples of by 1 when multipleOf is not set
	if notMultipleOf == 0 && explicitDenyInteger {
		notMultipleOf = 1
	}

	// Do nothing if there are no multipleOf constraints to reconcile
	// either implicitly or explicitly set
	if multipleOf == 0 && notMultipleOf == 0 {
		return nil
	}

	if multipleOf != 0 && notMultipleOf != 0 && multipleOf == notMultipleOf {
		warnField(metadata, "not/multipleOf", fmt.Errorf("cannot have 'multipleOf' and not 'multipleOf' set to the same value, %f, they are mutually exclusive", notMultipleOf))
	} else if multipleOf != 0 && notMultipleOf != 0 && math.Mod(multipleOf, notMultipleOf) == 0 {
		warnField(metadata, "not/multipleOf", fmt.Errorf("not/multipleOf cannot be a multiple of multipleOf"))
	} else if notMultipleOf != 0 && notMultipleOf <= infinitesimal {
		warnField(metadata, "not/multipleOf", fmt.Errorf("multipleOf is too small to enforce in a Not Clause"))
	} else if notMultipleOf != 0 {
		// We handle getting valid values of `not/multipleOf` by precomputing valid values and baking them into an enum
		// in order to create a valid range of values that can be generated.
		validValues := []float64{}
		if multipleOf == 0 {
			multipleOf = infinitesimal
		}

		min := util.GetZeroIfNil(min, lowerBound)
		max := util.GetZeroIfNil(max, upperBound)

		validValues, err := computeValidMultipleOfValues(metadata, min, max, multipleOf, notMultipleOf)
		if err != nil && alturnateMin != nil || alturnateMax != nil {
			alturnateMinVal := util.GetZeroIfNil(alturnateMin, lowerBound)
			alturnateMaxVal := util.GetZeroIfNil(alturnateMax, upperBound)

			validValues, err = computeValidMultipleOfValues(metadata, alturnateMinVal, alturnateMaxVal, multipleOf, notMultipleOf)

			// In the event both upper bound computations fail, we warn and exit given there is not
			// more we can reasonably do to find valid values
			if err != nil {
				warnField(metadata, "not/multipleOf", fmt.Errorf("unable to compute valid multipleOf values given the multipleOf and not/multipleOf constraints: %w", err))
				return nil
			}
		}

		newNode.MultipleOf = nil
		newNode.Maximum = nil
		newNode.Minimum = nil

		// Filter down existing enum to valid values if it exists
		if newNode.Enum != nil && len(*newNode.Enum) > 0 {
			filteredEnum := []interface{}{}

			for _, enumValue := range *newNode.Enum {
				if enumFloatValue, ok := enumValue.(float64); ok {
					if funk.ContainsFloat64(validValues, enumFloatValue) {
						filteredEnum = append(filteredEnum, enumValue)
					}
				}
			}

			if len(filteredEnum) == 0 {
				warnField(metadata, "not/multipleOf", fmt.Errorf("Unable to find any valid enum values given the multipleOf and not/multipleOf constraints"))
			}
			newNode.Enum = &filteredEnum
		} else {
			interfaceValues := make([]interface{}, len(validValues))
			for i, v := range validValues {
				interfaceValues[i] = v
			}
			newNode.Enum = &interfaceValues
		}
	} else {
		newNode.MultipleOf = node.MultipleOf
	}

	return nil

	//		Minimum          *float64 `json:"minimum,omitempty"`
	//		Maximum          *float64 `json:"maximum,omitempty"`
	//		ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	//		ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	//		MultipleOf       float64  `json:"multipleOf"`
}

func notApplyEnum(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error {
	if node.Enum == nil && notNode.Enum == nil {
		return nil
	}

	enum := util.GetZeroIfNil(node.Enum, []interface{}{})
	notEnum := util.GetZeroIfNil(notNode.Enum, []interface{}{})
	if len(enum) == 0 && len(notEnum) != 0 {
		notEnumJsonData := []string{}
		for _, val := range notEnum {
			notEnumJsonData = append(notEnumJsonData, util.MarshalJsonToString(val))
		}

		if node.Const == nil {
			constraintCollection.AddNotValueConstraint(notEnumJsonData)
			return nil
		}

		// Special edge case where const is set in the not enum
		constJsonData := util.MarshalJsonToString(node.Const)
		if funk.ContainsString(notEnumJsonData, constJsonData) {
			warnField(metadata, "not/enum", fmt.Errorf("cannot have 'not/enum' contain the same value as 'const'"))
			return nil
		}
	}

	enumJsonData := util.SafeMarshalListToJsonList(node.Enum)
	notEnumJsonData := util.SafeMarshalListToJsonList(notNode.Enum)

	differentValues, _ := funk.DifferenceString(enumJsonData, notEnumJsonData)

	if len(differentValues) == 0 {
		warnField(metadata, "not/enum", fmt.Errorf("cannot have 'not/enum' contain all the same values as 'enum'"))
		return nil
	}

	enumValues, ok := funk.Map(differentValues, util.UnmarshalJsonStringToMap).([]interface{})
	if !ok {
		warnField(metadata, "not/enum", fmt.Errorf("failed to parse enum values"))
		return nil
	}
	newNode.Enum = &enumValues

	return nil
}

// Handles
// - 'const'
func notApplyConst(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error {
	newNode.Const = node.Const

	if notNode.Const == nil {
		return nil
	}

	notNodeConstJson := util.MarshalJsonToString(notNode.Const)
	if node.Const != nil {
		nodeConstJson := util.MarshalJsonToString(node.Const)

		if nodeConstJson == notNodeConstJson {
			warnField(metadata, "not/const", fmt.Errorf("cannot have 'const' and 'not/const' set to the same value, they are mutually exclusive"))
			return nil
		}
	}

	if newNode.Enum != nil && len(*newNode.Enum) == 1 {
		enumJsonData := util.MarshalJsonToString((*newNode.Enum)[0])
		notConstJsonData := util.MarshalJsonToString(util.GetZeroIfNil(notNode.Const, nil))

		if enumJsonData == notConstJsonData {
			warnField(metadata, "not/const", fmt.Errorf("cannot have 'not/const' contain the same value as the only valid option for the 'enum' clause when 'enum' has only possible value"))
			return nil
		}
	}

	constraintCollection.AddNotValueConstraint([]string{notNodeConstJson})
	return nil
}

// Handles not case for array properties
// - minItems
// - maxItems
// - minContains
// - maxContains
// - items
// - uniqueItems
// - prefixItems
// - additionalItems
// - unevaluatedItems (Unsupported: Other details need confirming)
func notApplyArray(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error {
	newNode.MinItems, newNode.MaxItems = resolveBoundsInt(
		metadata,
		"minItems", "maxItems",
		node.MinItems, node.MaxItems,
		notNode.MinItems, notNode.MaxItems,
	)

	newNode.MinContains, newNode.MaxContains = resolveBoundsInt(
		metadata,
		"minContains", "maxContains",
		node.MinContains, node.MaxContains,
		notNode.MinContains, notNode.MaxContains,
	)

	if newNode.MinContains != nil && newNode.MaxItems != nil && *newNode.MinContains > *newNode.MaxItems {
		warnField(metadata, "not/minContains", fmt.Errorf("while resolving not clauses minContains cannot be greater than maxItems"))
		newNode.MinContains = newNode.MaxItems
	}

	newNode.Items = notMergeItemData(metadata, node.Items, notNode.Items)

	if notNode.UniqueItems != nil && *notNode.UniqueItems {
		warnField(metadata, "not/uniqueItems", fmt.Errorf("cannot have 'not/uniqueItems' set to true. It is very difficult to calculate items that match all clauses and are identical throughout the array"))
	}

	newNode.UniqueItems = resolveBool("not/uniqueItems", node.UniqueItems, notNode.UniqueItems)

	newNode.PrefixItems = notMergePrefixNodes(metadata, node.PrefixItems, notNode.PrefixItems, util.GetZeroIfNil(newNode.Items, itemsData{}))
	newNode.UnevaluatedItems = notMergeSchemaNodeOrFalse("/not/unevaluatedItems", metadata, node.UnevaluatedItems, notNode.UnevaluatedItems)
	newNode.Contains = notMergeSubNodePtr("/not/contains", metadata, node.Contains, notNode.Contains)

	return nil
}

// Handles not case for object properties
// - properties
// - patternProperties (Unsupported due to general ambiguity of regexes)
// - additionalProperties
// - required
// - minProperties
// - maxProperties
func notApplyObject(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error {

	newNode.MinProperties, newNode.MaxProperties = resolveBoundsInt(
		metadata,
		"minProperties", "maxProperties",
		node.MinProperties, node.MaxProperties,
		notNode.MinProperties, notNode.MaxProperties,
	)

	// Merge down properties
	properties := util.MapKeysToStringSlice(node.Properties, notNode.Properties)
	mergedProperties := map[string]schemaNode{}

	for _, propertyKey := range properties {
		mergedProperties[propertyKey] = notMergeSubNode(
			fmt.Sprintf("/not/properties/%s", propertyKey),
			metadata,
			util.GetObjectKeyOrDefault(node.Properties, propertyKey, schemaNode{}),
			util.GetObjectKeyOrDefault(notNode.Properties, propertyKey, schemaNode{}),
		)
	}

	// Merge down pattern properties
	warnUnsupportedField(metadata, "not/patternProperties", func() bool {
		return notNode.PatternProperties != nil && len(*notNode.PatternProperties) > 0
	})

	newNode.PatternProperties = node.PatternProperties

	// Additional properties
	newNode.AdditionalProperties = notMergeSchemaNodeOrFalse("/not/additionalProperties", metadata, node.AdditionalProperties, notNode.AdditionalProperties)

	// Required properties
	// Required is immutable so we have to handle coercing the object properties into a state where it wont have them
	newNode.Required = node.Required
	requiredProperties := util.GetZeroIfNil(node.Required, []string{})
	notRequiredProperties := util.GetZeroIfNil(notNode.Required, []string{})

	for _, notRequiredProperty := range notRequiredProperties {
		if funk.ContainsString(requiredProperties, notRequiredProperty) {
			warnField(metadata, "not/required", fmt.Errorf("cannot have 'required' and 'not/required' contain the same property '%s', they are mutually exclusive", notRequiredProperty))
			continue
		}

		_, exists := mergedProperties[notRequiredProperty]

		// If we are disallowing a node and it does not exist there is nothing to do
		if !exists {
			continue
		}

		delete(mergedProperties, notRequiredProperty)
	}

	constraintCollection.AddMustNotHaveProperties(notRequiredProperties)

	if len(mergedProperties) > 0 {
		newNode.Properties = &mergedProperties
	}

	return nil
}

func applyUnsupportedNotFields(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error {
	// Unsupported fields
	warnUnsupportedField(metadata, "not/dependentRequired", func() bool {
		return len(notNode.DependentRequired) > 0
	})

	warnUnsupportedField(metadata, "not/dependentSchemas", func() bool {
		return len(notNode.DependentSchemas) > 0
	})

	// If/Then/Else unsupported
	warnUnsupportedField(metadata, "not/if", func() bool {
		return notNode.If != nil
	})

	warnUnsupportedField(metadata, "not/then", func() bool {
		return notNode.Then != nil
	})

	warnUnsupportedField(metadata, "not/else", func() bool {
		return notNode.Else != nil
	})

	// No support for combinators in not nodes
	warnUnsupportedField(metadata, "not/allOf", func() bool {
		return notNode.AllOf != nil && len(*notNode.AllOf) > 0
	})

	warnUnsupportedField(metadata, "not/anyOf", func() bool {
		return notNode.AnyOf != nil && len(*notNode.AnyOf) > 0
	})

	warnUnsupportedField(metadata, "not/oneOf", func() bool {
		return notNode.OneOf != nil && len(*notNode.OneOf) > 0
	})

	return nil
}

// Merging subschemas from nodes and not nodes
func notMergeSubNode(scope string, metadata *parserMetadata, node schemaNode, notNode schemaNode) schemaNode {
	// Handle simplification of not nodes first
	metadata.ReferenceHandler.PushToPath(scope)
	defer metadata.ReferenceHandler.PopFromPath(scope)

	mergedNode, constraintCollection := notMerge(metadata, node, notNode)
	mergedNode.constraints = constraintCollection
	return mergedNode
}

func notMergeSubNodePtr(scope string, metadata *parserMetadata, nodePtr *schemaNode, notNodePtr *schemaNode) *schemaNode {
	if nodePtr == nil && notNodePtr == nil {
		return nil
	}

	node := util.GetZeroIfNil(nodePtr, schemaNode{})
	notNode := util.GetZeroIfNil(notNodePtr, schemaNode{})
	mergedNode := notMergeSubNode(scope, metadata, node, notNode)
	return &mergedNode
}

func notMergeItemData(metadata *parserMetadata, nodePtr *itemsData, notNodePtr *itemsData) *itemsData {
	if nodePtr == nil && notNodePtr == nil {
		return nil
	}

	node := util.GetZeroIfNil(nodePtr, itemsData{})
	notNode := util.GetZeroIfNil(notNodePtr, itemsData{})

	if node.DisallowAdditionalItems == true && notNode.DisallowAdditionalItems == true {
		warnField(metadata, "not/additionalItems", fmt.Errorf("cannot have 'additionalItems' and 'not/additionalItems' set to false, they are mutually exclusive"))
		return &node
	}

	// If we disallow additional items in the original node there is nothing to do
	if node.DisallowAdditionalItems == true {
		return &node
	}

	if node.Nodes != nil || notNode.Nodes != nil {
		warnField(metadata, "not/items [As Array]", fmt.Errorf("not merging 'items' as an array is not yet supported. Please use the more recent 'prefixItems' functionality instead if possible"))
	}

	if node.Node == nil && notNode.Node == nil {
		return nil
	}

	mergedItemDataNode := notMergeSubNode("/not/items", metadata,
		util.GetZeroIfNil(node.Node, schemaNode{}),
		util.GetZeroIfNil(notNode.Node, schemaNode{}),
	)

	return &itemsData{
		Node:                    &mergedItemDataNode,
		DisallowAdditionalItems: false,
	}
}

func notMergeSchemaNodeOrFalse(scope string, metadata *parserMetadata, nodePtr *schemaNodeOrFalse, notNodePtr *schemaNodeOrFalse) *schemaNodeOrFalse {
	if nodePtr == nil && notNodePtr == nil {
		return nil
	}

	node := util.GetZeroIfNil(nodePtr, schemaNodeOrFalse{})
	notNode := util.GetZeroIfNil(notNodePtr, schemaNodeOrFalse{})

	if node.IsFalse && notNode.IsFalse {
		warnField(metadata, "", fmt.Errorf("cannot have schema node and not schema node both set to false, they are mutually exclusive"))
		return &node
	}

	if node.IsFalse {
		return &node
	}

	mergedNode := notMergeSubNode(scope, metadata,
		util.GetZeroIfNil(node.Schema, schemaNode{}),
		util.GetZeroIfNil(notNode.Schema, schemaNode{}),
	)

	return &schemaNodeOrFalse{
		Schema: &mergedNode,
	}
}

func notMergePrefixNodes(metadata *parserMetadata, nodePrefixItemsPtr *[]schemaNode, notNodePrefixItemsPtr *[]schemaNode, additionalItems itemsData) *[]schemaNode {
	if nodePrefixItemsPtr == nil && notNodePrefixItemsPtr == nil {
		return nil
	}

	nodePrefixItems := util.GetZeroIfNil(nodePrefixItemsPtr, []schemaNode{})
	notNodePrefixItems := util.GetZeroIfNil(notNodePrefixItemsPtr, []schemaNode{})

	length := int(math.Max(float64(len(nodePrefixItems)), float64(len(notNodePrefixItems))))
	mergedPrefixItems := []schemaNode{}

	for i := 0; i < length; i++ {
		path := fmt.Sprintf("/not/prefixItems/%d", i)
		mergedPrefixItems = append(mergedPrefixItems, notMergeSubNode(path, metadata,
			util.GetIndexOrDefault(nodePrefixItems, i, util.GetZeroIfNil(additionalItems.Node, schemaNode{})),
			util.GetIndexOrDefault(notNodePrefixItems, i, schemaNode{}),
		))
	}

	return &mergedPrefixItems
}

// Resolves the type for a node given the not node types
func resolveType(node schemaNode, notNode schemaNode) (multipleType, error) {
	nodeTypes := getNodeTypes(node, false)
	notNodeTypes := getNodeTypes(notNode, true)
	candidateNodeTypes := []string{}

	for _, nodeType := range nodeTypes {
		if funk.ContainsString(notNodeTypes, nodeType) {
			continue
		}

		candidateNodeTypes = append(candidateNodeTypes, nodeType)
	}

	if len(candidateNodeTypes) == 0 {
		return multipleType{}, fmt.Errorf("invalid schema: Failed to find candidate type that satisfies node types %s while excluding %s", nodeTypes, notNodeTypes)
	}

	return newMultipleTypeFromSlice(candidateNodeTypes), nil
}

// Gets the diff between disallowed and allowed types for a given node
// returning the intersection between the two where available types work
// returning an error where that is not possible
func getNodeTypes(node schemaNode, notType bool) []string {
	if node.Type == nil {
		if notType {
			// If no type is given we assume all types are restricting
			// no types explicitly
			return []string{}
		}

		return typeAll
	} else if node.Type.SingleType != "" {
		return []string{node.Type.SingleType}
	} else if len(node.Type.MultipleTypes) != 0 {
		return node.Type.MultipleTypes
	} else if notType {
		return []string{}
	} else {
		return typeAll
	}
}

func nodeTypeContains(node *schemaNode, candidateType string) bool {
	if node.Type == nil {
		return false
	}

	return node.Type.SingleType == candidateType || (node.Type.MultipleTypes != nil && funk.ContainsString(node.Type.MultipleTypes, candidateType))
}

// Given we shift the bounds +1 or -1 we need to ensure we don't overflow
const MinInt = math.MinInt32 + 1
const MaxInt = math.MaxInt32 - 1

func resolveBoundsInt(metadata *parserMetadata,
	minFieldName string,
	maxFieldName string,
	minPtr *int, maxPtr *int,
	notMinPtr *int, notMaxPtr *int,
) (*int, *int) {
	// Edge case: No not constraints
	if notMinPtr == nil && notMaxPtr == nil {
		return minPtr, maxPtr
	}

	// Edge case: No original bounds -> Select upper bound
	if minPtr == nil && maxPtr == nil {
		if notMinPtr != nil {
			newMax := *notMinPtr - 1
			return nil, maxToNil(newMax)
		}

		if notMaxPtr != nil {
			newMin := *notMaxPtr + 1
			return minToNil(newMin), nil
		}

		return nil, nil
	}

	// Inclusive bounds
	x1 := util.GetZeroIfNil(minPtr, MinInt)
	x2 := util.GetZeroIfNil(maxPtr, MaxInt)

	// Not bounds are inverted (notMax -> min, notMin -> max)
	y1 := util.GetZeroIfNil(notMinPtr, MinInt)
	y2 := util.GetZeroIfNil(addIfNotNilInt(notMaxPtr, 1), MaxInt)

	// No bounds overlap return original
	if y2 < x1 || y1 > x2 {
		return minToNil(x1), maxToNil(x2)
	}

	// No valid range
	if y1 <= x1 && y2 >= x2 {
		warnField(metadata, maxFieldName, fmt.Errorf("while resolving not clauses %s and %s, no valid range exists that include [%d, %d] that excludes [%d, %d]", minFieldName, maxFieldName, x1, y1, y2, y1))
		return minPtr, maxPtr
	}

	// Check lower side for overlap
	if y1 <= x1 && y2 < x2 {
		newMin := y2 + 1
		return minToNil(newMin), maxToNil(x2)
	}

	// Check upper overlap
	if y1 > x1 && y2 >= x2 {
		newMax := y1 - 1
		return minToNil(x1), maxToNil(newMax)
	}

	// Overlap in middle - prefer lower overlap
	newMax := y1 - 1
	return minToNil(x1), maxToNil(newMax)
}

// Resolve float64 bounds with exclusive/inclusive handling for not nodes and original nodes
func resolveBoundsFloat64(metadata *parserMetadata,
	minFieldName string,
	maxFieldName string,
	minPtr *float64, minExclusivePtr *float64,
	maxPtr *float64, maxExclusivePtr *float64,
	notMinPtr *float64, notMinExclusivePtr *float64,
	notMaxPtr *float64, notMaxExclusivePtr *float64,
	offsetIncrement float64,
) (*float64, *float64, *float64, *float64) {
	x1 := util.MaxFloatPtr(minPtr, addIfNotNilFloat64(minExclusivePtr, offsetIncrement))
	x2 := util.MinFloatPtr(maxPtr, addIfNotNilFloat64(maxExclusivePtr, -offsetIncrement))

	y1 := util.MaxFloatPtr(notMinPtr, addIfNotNilFloat64(notMinExclusivePtr, offsetIncrement))
	y2 := util.MinFloatPtr(notMaxPtr, addIfNotNilFloat64(notMaxExclusivePtr, -offsetIncrement))

	// Edge case: No not constraints
	if y1 == nil && y2 == nil {
		return x1, x2, nil, nil
	}

	// Edge case: No original bounds -> Select upper bound
	if x1 == nil && x2 == nil {
		if y1 != nil {
			newMax := *y1 - offsetIncrement
			return nil, &newMax, nil, nil
		}

		if y2 != nil {
			newMin := *y2 + offsetIncrement
			return &newMin, nil, nil, nil
		}

		return nil, nil, nil, nil
	}

	// Resolve to numeric values
	x1Val := util.GetZeroIfNil(x1, lowerBound)
	x2Val := util.GetZeroIfNil(x2, upperBound)

	y1Val := util.GetZeroIfNil(y1, lowerBound)
	y2Val := util.GetZeroIfNil(y2, upperBound)

	// No bounds overlap return original
	if y2Val < x1Val || y1Val > x2Val {
		return minToNilFloat64(x1Val, offsetIncrement), maxToNilFloat64(x2Val, offsetIncrement), nil, nil
	}

	// No valid range
	if y1Val <= x1Val && y2Val >= x2Val {
		warnField(metadata, maxFieldName, fmt.Errorf("while resolving not clauses %s and %s, no valid range exists that include [%f, %f] that excludes [%f, %f]", minFieldName, maxFieldName, x1Val, y1Val, y2Val, y1Val))
		return minToNilFloat64(x1Val, offsetIncrement), maxToNilFloat64(x2Val, offsetIncrement), nil, nil
	}

	// Check lower side for overlap
	if y1Val <= x1Val && y2Val < x2Val {
		newMin := y2Val + offsetIncrement
		return minToNilFloat64(newMin, offsetIncrement), maxToNilFloat64(x2Val, offsetIncrement), nil, nil
	}

	// Check upper overlap
	if y1Val > x1Val && y2Val >= x2Val {
		newMax := y1Val - offsetIncrement
		return minToNilFloat64(x1Val, offsetIncrement), maxToNilFloat64(newMax, offsetIncrement), nil, nil
	}

	// Overlap in middle give both bounds
	lowerBoundMin := x1Val
	lowerBoundMax := y1Val - offsetIncrement

	upperBoundMin := y2Val + offsetIncrement
	upperBoundMax := x2Val
	return minToNilFloat64(lowerBoundMin, offsetIncrement), maxToNilFloat64(lowerBoundMax, offsetIncrement), minToNilFloat64(upperBoundMin, offsetIncrement), maxToNilFloat64(upperBoundMax, offsetIncrement)

}

// Helper to convert min/max int to nil where unbounded
// We use a small buffer to avoid edge cases to do with shifted bounds
func minToNil(min int) *int {
	if min <= MinInt+10 {
		return nil
	}

	return &min
}

func maxToNil(max int) *int {
	if max >= MaxInt-10 {
		return nil
	}
	return &max
}

func minToNilFloat64(min float64, offsetIncrement float64) *float64 {
	if min <= lowerBound+(offsetIncrement*10) {
		return nil
	}

	return &min
}

func maxToNilFloat64(max float64, offsetIncrement float64) *float64 {
	if max >= upperBound-(offsetIncrement*10) {
		return nil
	}
	return &max
}

func addIfNotNilFloat64(value *float64, addition float64) *float64 {
	if value == nil {
		return nil
	}

	newValue := *value + float64(addition)
	return &newValue
}

func addIfNotNilInt(value *int, addition int) *int {
	if value == nil {
		return nil
	}

	newValue := *value + int(addition)
	return &newValue
}

func computeValidMultipleOfValues(metadata *parserMetadata, min float64, max float64, multipleOf float64, notMultipleOf float64) ([]float64, error) {
	const maximumIterations = 10000
	const maximumValidCandidates = 200

	iterations := 0
	validValues := []float64{}

	for value := min; value <= max; value += multipleOf {
		if math.Mod(value, notMultipleOf) != 0 {
			validValues = append(validValues, value)
		}
		iterations++

		if iterations >= maximumIterations || len(validValues) >= maximumValidCandidates {
			break
		}
	}

	if len(validValues) == 0 {
		return nil, fmt.Errorf("unable to compute any valid values for multipleOf %f excluding not/multipleOf %f in range [%f, %f]", multipleOf, notMultipleOf, min, max)
	}
	return validValues, nil
}

func resolveBool(fieldName string, value *bool, notValue *bool) *bool {
	if value == nil && notValue == nil {
		return nil
	}

	if value != nil && notValue != nil && *value == *notValue {
		warnField(nil, fieldName, fmt.Errorf("cannot have boolean value and its negation set to the same value"))
		return value
	}

	return util.GetPtr(value, notValue)
}

// Warnings
func warnUnsupportedField(metadata *parserMetadata, fieldName string, fieldEmptyFunc func() bool) {
	if fieldEmptyFunc() {
		warnField(metadata, fieldName, fmt.Errorf("not for '%s' not yet supported", fieldName))
	}
}

func warnField(metadata *parserMetadata, fieldName string, err error) {
	if err != nil {
		errorPath := fmt.Sprintf("/%s", fieldName)
		metadata.Errors.AddErrorWithSubpath(errorPath, err)
	}
}
