package chaff

import (
	"fmt"
	"math"

	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/thoas/go-funk"
)

type (
	notMergeFunction func(metadata *parserMetadata, newNode *schemaNode, constraintCollection *constraintCollection, node schemaNode, notNode schemaNode) error
)

// N.b Order matters here
var notMergeFunctions = []notMergeFunction{
	notApplyType,
	notApplyString,
	notApplyNumberAndInteger,
	notApplyEnum,
	notApplyConst,
}

// Parses the "not" type of a schema
// Example:
// {
//   "not": {"type": "null"}
// }

// Strategy:
//   Recursively coerces the node tree to account for the "not" node where possible
//   and applying constraints post the value being generated if not
//   then hands off the node for regular parsing

func parseNot(node schemaNode, metadata *parserMetadata) (Generator, error) {
	// Flatten the not node structure
	metadata.ReferenceHandler.PushToPath("not")
	notNode, err := mergeSchemaNodes(metadata, schemaNode{}, *node.Not)
	metadata.ReferenceHandler.PopFromPath("not")

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
	//newNode.MinItems, newNode.MaxItems = resolveBoundsInt(
	//	metadata,
	//	"minItems", "maxItems",
	//	node.MinItems, node.MaxItems,
	//	notNode.MinItems, notNode.MaxItems,
	//)
	//
	//newNode.MinContains, newNode.MaxContains = resolveBoundsInt(
	//	metadata,
	//	"minContains", "maxContains",
	//	node.MinContains, node.MaxContains,
	//	notNode.MinContains, notNode.MaxContains,
	//)
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
	if err != nil {
		metadata.Errors[metadata.ReferenceHandler.CurrentPath] = err
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
	// Error cases
	multipleOf := util.GetZeroIfNil(node.MultipleOf, 0)
	notMultipleOf := util.GetZeroIfNil(notNode.MultipleOf, 0)
	if multipleOf != 0 && notMultipleOf != 0 && multipleOf == notMultipleOf {
		warnField(metadata, "not/multipleOf", fmt.Errorf("cannot have 'multipleOf' and not 'multipleOf' set to the same value, %f, they are mutually exclusive", notMultipleOf))
	} else if multipleOf != 0 && notMultipleOf != 0 && math.Mod(multipleOf, notMultipleOf) == 0 {
		warnField(metadata, "not/multipleOf", fmt.Errorf("not/multipleOf cannot be a multiple of multipleOf"))
	} else if notMultipleOf != 0 && notMultipleOf <= infinitesimal {
		warnField(metadata, "not/multipleOf", fmt.Errorf("multipleOf is too small to enforce in a Not Clause"))
	} else if multipleOf == 0 && notMultipleOf != 0 {
		// @todo Search for how to do this
	}

	newNode.Minimum = util.GetFloatPtr(node.Minimum, node.ExclusiveMinimum)
	newNode.Maximum = util.GetFloatPtr(node.Maximum, node.ExclusiveMaximum)

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
		notEnumJsonData := funk.Map(notNode.Enum, util.MarshalJsonToString).([]string)

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

	enumJsonData := funk.Map(node.Enum, util.MarshalJsonToString).([]string)
	notEnumJsonData := funk.Map(notNode.Enum, util.MarshalJsonToString).([]string)

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

// Merge specific type fields
// Shared Properties
//		Type   multipleType `json:"type"`
//		Length int          `json:"length"` // Shared by String and Array
//
//		// Object Properties
//		Properties           map[string]schemaNode `json:"properties"`
//		AdditionalProperties additionalData        `json:"additionalProperties"`
//		PatternProperties    map[string]schemaNode `json:"patternProperties"`
//		MinProperties        int                   `json:"minProperties"`
//		MaxProperties        int                   `json:"maxProperties"`
//		Required             []string              `json:"required"`
//
//		// String Properties
//		Pattern   string `json:"pattern"`
//		Format    string `json:"format"`
//		MinLength int    `json:"minLength"`
//		MaxLength int    `json:"maxLength"`
//
//		// Number Properties
//		Minimum          *float64 `json:"minimum,omitempty"`
//		Maximum          *float64 `json:"maximum,omitempty"`
//		ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
//		ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
//		MultipleOf       float64  `json:"multipleOf"`
//
//		// Array Properties
//		Items    itemsData `json:"items"`
//		MinItems int       `json:"minItems"` // N Done
//		MaxItems int       `json:"maxItems"` // N Done
//
//		Contains    *schemaNode `json:"contains"`
//		MinContains int         `json:"minContains"` // N Done
//		MaxContains int         `json:"maxContains"` // N Done
//
//		PrefixItems      []schemaNode       `json:"prefixItems"`
//		AdditionalItems  *schemaNodeOrFalse `json:"additionalItems"`
//		UnevaluatedItems *schemaNodeOrFalse `json:"unevaluatedItems"`
//		UniqueItems      bool               `json:"uniqueItems"`
//
//		// Enum Properties
//		Enum []interface{} `json:"enum"`
//
//		// Constant Properties
//		Const interface{} `json:"const"`
//
//		// Combination Properties
//		// TODO: Implement these
//		Not   *schemaNode  `json:"not"`
//		AllOf []schemaNode `json:"allOf"`
//		AnyOf []schemaNode `json:"anyOf"`
//		OneOf []schemaNode `json:"oneOf"`
//
//		// Reference Operator
//		Ref         string                `json:"$ref"`
//		Id          string                `json:"$id"`
//		Defs        map[string]schemaNode `json:"$defs"`
//		Definitions map[string]schemaNode `json:"definitions"`
//
//		// Unsupported Properties
//		If                *schemaNode           `json:"if"`
//		Then              *schemaNode           `json:"then"`
//		Else              *schemaNode           `json:"else"`
//		DependentRequired map[string][]string   `json:"dependentRequired"`
//		DependentSchemas  map[string]schemaNode `json:"dependentSchemas"`

func resolveType(node schemaNode, notNode schemaNode) (multipleType, error) {
	nodeTypes := getNodeTypes(node, true)
	notNodeTypes := getNodeTypes(notNode, false)
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

	return newMultipleTypeFromSlice(nodeTypes), nil
}

// Gets the diff between disallowed and allowed types for a given node
// returning the intersection between the two where available types work
// returning an error where that is not possible
func getNodeTypes(node schemaNode, infer bool) []string {
	if node.Type == nil {
		if infer {
			return []string{inferType(node)}
		} else {
			return typeAll
		}
	} else if node.Type.SingleType != "" {
		return []string{node.Type.SingleType}
	} else if len(node.Type.MultipleTypes) != 0 {
		return node.Type.MultipleTypes
	} else if infer {
		inferredType := inferType(node)
		if inferredType != typeUnknown {
			return []string{inferredType}
		}
		return typeAll
	} else {
		return []string{}
	}
}

// Handle bound checks for minX -> maxX
func resolveBoundsInt(metadata *parserMetadata,
	minFieldName string,
	maxFieldName string,
	minPtr *int, maxPtr *int,
	notMinPtr *int, notMaxPtr *int,
) (*int, *int) {

	var resolvedMin int = 0
	var resolvedMax int = 0
	max := util.GetZeroIfNil(maxPtr, 0)
	min := util.GetZeroIfNil(minPtr, 0)
	notMax := util.GetZeroIfNil(notMaxPtr, 0)
	notMin := util.GetZeroIfNil(notMinPtr, 0)

	if notMax == 0 && notMin == 0 {
		return &min, &max
	}

	if notMax != 0 && min != 0 && min > notMax {
		warnField(metadata, minFieldName, fmt.Errorf("minimum set in '%s' means the 'not' maximum given in '%s' cannot be satisfied defaulting to '%s''s value", minFieldName, maxFieldName, minFieldName))
		resolvedMin = min
	} else {
		resolvedMin = int(math.Max(float64(notMax), float64(min)))
	}

	if notMin != 0 && max != 0 && max > notMin {
		warnField(metadata, maxFieldName, fmt.Errorf("minimum set in '%s' means the 'not' minimum given in '%s' cannot be satisfied defaulting to '%s''s value", maxFieldName, minFieldName, maxFieldName))
		resolvedMax = max
	} else if notMin != 0 && max != 0 {
		resolvedMax = int(math.Min(float64(notMin), float64(max)))
	} else if notMin != 0 {
		resolvedMax = notMin
	} else if max != 0 {
		resolvedMax = max
	} else {
		resolvedMax = 0
	}

	if resolvedMax > resolvedMin {
		warnField(metadata, maxFieldName, fmt.Errorf("while resolving not clauses %s and %s resolve to values where the maximum is larger than the minimum value [min: %d, max: %d]. Setting both to min value", minFieldName, maxFieldName, resolvedMin, resolvedMax))
		resolvedMax = resolvedMin
	}
	return &resolvedMin, &resolvedMax
}

// Warnings
func warnUnsupportedField(metadata *parserMetadata, fieldName string, fieldEmptyFunc func() bool) {
	if fieldEmptyFunc() {
		warnField(metadata, fieldName, fmt.Errorf("not for '%s' not yet supported", fieldName))
	}
}

func warnField(metadata *parserMetadata, fieldName string, err error) {
	if err != nil {
		errorPath := fmt.Sprintf("%s/%s", metadata.ReferenceHandler.CurrentPath, fieldName)
		metadata.Errors[errorPath] = err
	}
}
