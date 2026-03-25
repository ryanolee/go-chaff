package chaff

import (
	"fmt"
	"math"

	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/thoas/go-funk"
)

func newEmptySchemaNode() schemaNode {
	return schemaNode{}
}

// Merges all sub properties of a given node
func mergeSchemaNodes(metadata *parserMetadata, nodes ...schemaNode) (schemaNode, error) {
	mergedNode := newEmptySchemaNode()

	for _, node := range nodes {
		node, resolvedReference := mergeResolveNodeRef(metadata, node)

		// If the resolved node contains allOf, flatten it by recursively
		// merging the allOf sub-schemas into the node. This prevents allOf
		// from propagating up to the merged result, which would cause
		// parseAllOf → merge → parseAllOf infinite recursion.
		// The current ref is on the HasResolved stack, so cycles within
		// the flattened allOf are correctly detected and short-circuited.
		if node.AllOf != nil && resolvedReference {
			subNodes := append([]schemaNode{}, *node.AllOf...)
			node.AllOf = nil
			subNodes = append(subNodes, node)
			flatNode, err := mergeSchemaNodes(metadata, subNodes...)
			if err != nil {
				warnConfigMergeError(metadata, "allOf_flatten", err)
			} else {
				node = flatNode
			}
		}

		// Merge Type
		mergedNode.Type = mergeSchemaTypes(mergedNode.Type, node.Type)

		mergedNode = mergeEnumAndConst(metadata, mergedNode, node)

		// Merge object properties
		mergedNode.Properties = mergeProperties(metadata, mergedNode.Properties, node.Properties)
		mergedNode.AdditionalProperties = mergeNodeOrFalse(metadata, mergedNode.AdditionalProperties, node.AdditionalProperties, "additionalProperties")
		mergedNode.PatternProperties = mergePatternProperties(metadata, mergedNode.PatternProperties, node.PatternProperties)

		// Merge array items
		mergedNode.PrefixItems = mergePrefixItems(metadata, mergedNode.PrefixItems, node.PrefixItems)
		mergedNode.Items = mergeItemsData(metadata, mergedNode.Items, node.Items)
		mergedNode.AdditionalItems = mergeNodeOrFalse(metadata, mergedNode.AdditionalItems, node.AdditionalItems, "additionalItems")
		mergedNode.UnevaluatedItems = mergeNodeOrFalse(metadata, mergedNode.UnevaluatedItems, node.UnevaluatedItems, "unevaluatedItems")
		mergedNode.Contains = mergeSchemaPtrs(metadata, "contains", mergedNode.Contains, node.Contains)

		// Merge combinators
		mergedNode.OneOf = util.MergeSlicePtrs(mergedNode.OneOf, node.OneOf)
		mergedNode.AnyOf = util.MergeSlicePtrs(mergedNode.AnyOf, node.AnyOf)
		mergedNode.AllOf = util.MergeSlicePtrs(mergedNode.AllOf, node.AllOf)

		// Merge conditionals
		mergedNode = mergeNot(mergedNode, node)
		mergedNode = mergeIf(metadata, mergedNode, node)

		// Merge simple properties
		mergedNode = mergeSchemaNodeSimpleProperties(metadata, mergedNode, node)

		// Propagate not-constraints from prior processing (e.g., notMerge sets
		// .constraints on sub-nodes). Accumulate them so that sequential merges
		// don't lose earlier exclusions.
		if node.constraints != nil {
			if mergedNode.constraints == nil {
				mergedNode.constraints = node.constraints
			} else {
				mergedNode.constraints.Merge(node.constraints)
			}
		}

		// If the node had a circular $ref that couldn't be inlined during merge,
		// propagate it to the merged result so parseSchemaNode can create a
		// referenceGenerator instead of treating it as an empty "any type" node.
		if node.Ref != nil {
			mergedNode.Ref = node.Ref
		}

		if resolvedReference {
			// Pop the reference resolution and set the document being resolved back to the previous one
			metadata.ReferenceResolver.PopRefResolution()
			document, _ := metadata.ReferenceResolver.GetCurrentResolution()
			metadata.DocumentResolver.SetDocumentBeingResolved(document)
		}
	}

	return flattenResidualAllOf(metadata, mergedNode)
}

// mergeResolveNodeRef resolves any $ref on the node by following the reference
// chain, pushing the resolution onto the stack, and returning the resolved node.
// The second return value indicates whether a reference was resolved (and thus
// needs to be popped by the caller after processing).
func mergeResolveNodeRef(metadata *parserMetadata, node schemaNode) (schemaNode, bool) {
	for node.Ref != nil {
		documentId, path, err := metadata.DocumentResolver.ResolveDocumentIdAndPath(*node.Ref)

		if err != nil {
			warnConfigMergeError(metadata, "/$ref", fmt.Errorf("failed to resolve document for ref [%s]: %w", *node.Ref, err))
			node.Ref = nil
			return node, false
		}
		// Give up on the node if it is a circular reference.
		// Preserve the $ref so that parseSchemaNode routes it through
		// parseReference → referenceGenerator, which handles cycles at
		// generation time with depth limits. Without this the node would
		// be empty and get parsed as "any type".
		if metadata.ReferenceResolver.HasResolved(documentId, path) {
			return node, false
		}

		finalRefPath := fmt.Sprintf("%s%s", documentId, path)
		node.Ref = &finalRefPath

		refNode, err := mergeResolveReference(metadata, node)
		if err != nil {
			warnConfigMergeError(metadata, "/$ref", fmt.Errorf("failed to resolve ref [%s]: %w", *node.Ref, err))
			node.Ref = nil
			return node, false
		}

		metadata.ReferenceResolver.PushRefResolution(documentId, path)
		return refNode, true
	}

	return node, false
}

// collectEnumValues returns the effective set of allowed values for a node.
// A const is treated as a single-value enum; if both const and enum are
// present the enum is returned (the const is redundant). Returns nil when
// the node has neither const nor enum.
func collectEnumValues(node schemaNode) *[]interface{} {
	if node.Enum != nil {
		return node.Enum
	}
	if node.Const != nil {
		return &[]interface{}{*node.Const}
	}
	return nil
}

// mergeEnumAndConst merges the allowed-value constraints (enum / const) from
// the source node into the merged node using allOf intersection semantics.
// Both sides are normalised to enum slices first (const → single-value enum),
// then intersected. The result is written back as either a const (one value)
// or an enum (many values). An empty intersection produces a merge error and
// clears both fields.
func mergeEnumAndConst(metadata *parserMetadata, mergedNode schemaNode, node schemaNode) schemaNode {
	left := collectEnumValues(mergedNode)
	right := collectEnumValues(node)

	// Nothing to merge — neither side constrains allowed values.
	if left == nil && right == nil {
		return mergedNode
	}

	// Only one side has values — adopt it directly.
	var result *[]interface{}
	if left == nil {
		result = right
	} else if right == nil {
		result = left
	} else {
		result = intersectEnumSlices(left, right)
		if len(*result) == 0 {
			warnConfigMergeError(metadata, "enum/const", fmt.Errorf("enum/const intersection is empty left: %v, right: %v", left, right))
			mergedNode.Enum = nil
			mergedNode.Const = nil
			return mergedNode
		}
	}

	// Single value → const, multiple values → enum.
	if len(*result) == 1 {
		mergedNode.Const = &(*result)[0]
		mergedNode.Enum = nil
	} else {
		mergedNode.Enum = result
		mergedNode.Const = nil
	}

	return mergedNode
}

// mergeProperties merges two sets of object properties. When both sides define
// the same key the values are recursively merged; new keys are assigned directly
// so that any $ref they contain is preserved for lazy resolution.
func mergeProperties(metadata *parserMetadata, mergedPtr *map[string]schemaNode, nodePtr *map[string]schemaNode) *map[string]schemaNode {
	if nodePtr == nil {
		return mergedPtr
	}

	nodeProperties := util.GetZeroIfNil(nodePtr, map[string]schemaNode{})
	mergedProperties := util.GetZeroIfNil(mergedPtr, map[string]schemaNode{})

	for key, value := range nodeProperties {
		if existing, exists := mergedProperties[key]; exists {
			merged, err := mergeSchemaNodes(metadata, existing, value)
			if err != nil {
				errPath := fmt.Sprintf("/properties/%s/config_merge_error", key)
				metadata.Errors.AddErrorWithSubpath(errPath, err)
				warnConfigMergeError(metadata, fmt.Sprintf("properties/%s", key), err)
			}
			mergedProperties[key] = merged
		} else {
			mergedProperties[key] = value
		}
	}

	return &mergedProperties
}

// mergePatternProperties merges two sets of pattern properties using the same
// strategy as mergeProperties.
func mergePatternProperties(metadata *parserMetadata, mergedPtr *map[string]schemaNode, nodePtr *map[string]schemaNode) *map[string]schemaNode {
	if nodePtr == nil {
		return mergedPtr
	}

	nodePatternProperties := util.GetZeroIfNil(nodePtr, map[string]schemaNode{})
	mergedPatternProperties := util.GetZeroIfNil(mergedPtr, map[string]schemaNode{})

	for key, value := range nodePatternProperties {
		if existing, exists := mergedPatternProperties[key]; exists {
			merged, err := mergeSchemaNodes(metadata, existing, value)
			if err != nil {
				warnConfigMergeError(metadata, fmt.Sprintf("patternProperties/%s", key), err)
			}
			mergedPatternProperties[key] = merged
		} else {
			mergedPatternProperties[key] = value
		}
	}

	return &mergedPatternProperties
}

// mergePrefixItems merges two prefix-item arrays by position. Where both sides
// have an entry at the same index they are recursively merged; otherwise the
// entry from whichever side has it is kept.
func mergePrefixItems(metadata *parserMetadata, mergedPtr *[]schemaNode, nodePtr *[]schemaNode) *[]schemaNode {
	if nodePtr == nil {
		return mergedPtr
	}

	prefixItems := util.GetZeroIfNil(nodePtr, []schemaNode{})
	mergedPrefixItems := util.GetZeroIfNil(mergedPtr, []schemaNode{})
	length := int(math.Max(float64(len(prefixItems)), float64(len(mergedPrefixItems))))
	newMergedPrefixItems := make([]schemaNode, length)

	for i := 0; i < length; i++ {
		hasMerged := i < len(mergedPrefixItems)
		hasNew := i < len(prefixItems)
		if hasMerged && hasNew {
			merged, err := mergeSchemaNodes(metadata, mergedPrefixItems[i], prefixItems[i])
			if err != nil {
				warnConfigMergeError(metadata, fmt.Sprintf("prefixItems/%d", i), err)
			}
			newMergedPrefixItems[i] = merged
		} else if hasNew {
			newMergedPrefixItems[i] = prefixItems[i]
		} else {
			newMergedPrefixItems[i] = mergedPrefixItems[i]
		}
	}

	return &newMergedPrefixItems
}

// mergeNot accumulates "not" sub-schemas from both sides. When multiple allOf
// branches each contribute a "not", they must be processed independently because
// not(A) AND not(B) ≠ not(merge(A, B)). Already-collected mergedNot entries from
// sub-merges are also propagated.
func mergeNot(mergedNode schemaNode, node schemaNode) schemaNode {
	if node.Not != nil {
		if mergedNode.Not != nil || len(mergedNode.mergedNot) > 0 {
			// We already have not node(s) — collect separately
			if mergedNode.Not != nil {
				mergedNode.mergedNot = append(mergedNode.mergedNot, mergedNode.Not)
				mergedNode.Not = nil
			}
			mergedNode.mergedNot = append(mergedNode.mergedNot, node.Not)
		} else {
			mergedNode.Not = node.Not
		}
	}
	// Propagate already-collected not nodes from sub-merges. When incoming
	// mergedNot is non-empty, ensure any existing mergedNode.Not also moves
	// into the collection so they're all handled uniformly.
	if len(node.mergedNot) > 0 {
		if mergedNode.Not != nil {
			mergedNode.mergedNot = append(mergedNode.mergedNot, mergedNode.Not)
			mergedNode.Not = nil
		}
		mergedNode.mergedNot = append(mergedNode.mergedNot, node.mergedNot...)
	}
	return mergedNode
}

// flattenResidualAllOf iteratively flattens any allOf that survived the main
// merge loop (from inline schemas that weren't resolved through $ref). This
// guarantees callers always receive a node with allOf fully absorbed.
func flattenResidualAllOf(metadata *parserMetadata, mergedNode schemaNode) (schemaNode, error) {
	for mergedNode.AllOf != nil && len(*mergedNode.AllOf) > 0 {
		metadata.ParseDepth++
		if metadata.ParseDepth > metadata.ParserOptions.MaxParseDepth {
			metadata.ParseDepth--
			break
		}
		extraNodes := append([]schemaNode{}, *mergedNode.AllOf...)
		mergedNode.AllOf = nil
		extraNodes = append(extraNodes, mergedNode)
		var flatErr error
		mergedNode, flatErr = mergeSchemaNodes(metadata, extraNodes...)
		metadata.ParseDepth--
		if flatErr != nil {
			return mergedNode, flatErr
		}
	}

	return mergedNode, nil
}

func mergeSchemaNodeSimpleProperties(metadata *parserMetadata, baseNode schemaNode, otherNode schemaNode) schemaNode {

	warnIfBothSetAndAreDifferent(metadata, "length", otherNode.Length, baseNode.Length)
	baseNode.Length = util.GetPtr(otherNode.Length, baseNode.Length)

	// Merge simple int properties
	baseNode.MinProperties = util.MaxFloatPtr(otherNode.MinProperties, baseNode.MinProperties)
	baseNode.MaxProperties = util.MinFloatPtr(otherNode.MaxProperties, baseNode.MaxProperties)
	baseNode.MinItems = util.MaxFloatPtr(otherNode.MinItems, baseNode.MinItems)
	baseNode.MaxItems = util.MinFloatPtr(otherNode.MaxItems, baseNode.MaxItems)
	baseNode.MinContains = util.MaxFloatPtr(otherNode.MinContains, baseNode.MinContains)
	baseNode.MaxContains = util.MinFloatPtr(otherNode.MaxContains, baseNode.MaxContains)
	baseNode.MinLength = util.MaxFloatPtr(otherNode.MinLength, baseNode.MinLength)
	baseNode.MaxLength = util.MinFloatPtr(otherNode.MaxLength, baseNode.MaxLength)

	// Merge simple float properties
	baseNode.Minimum = util.MaxFloatPtr(otherNode.Minimum, baseNode.Minimum)
	baseNode.Maximum = util.MinFloatPtr(otherNode.Maximum, baseNode.Maximum)
	baseNode.ExclusiveMinimum = util.MaxFloatPtr(otherNode.ExclusiveMinimum, baseNode.ExclusiveMinimum)
	baseNode.ExclusiveMaximum = util.MinFloatPtr(otherNode.ExclusiveMaximum, baseNode.ExclusiveMaximum)
	baseNode.MultipleOf = util.MultiplyIfPossibleAndNotMultipleOfFloat64(otherNode.MultipleOf, baseNode.MultipleOf)

	// Merge simple string properties
	warnIfBothSetAndAreDifferent(metadata, "pattern", baseNode.Pattern, otherNode.Pattern)
	baseNode.Pattern = util.GetPtr(otherNode.Pattern, baseNode.Pattern)

	warnIfBothSetAndAreDifferent(metadata, "format", baseNode.Format, otherNode.Format)
	baseNode.Format = util.GetPtr(otherNode.Format, baseNode.Format)

	// Simple slice properties
	baseNode.Required = util.MergeSlicePtrs(baseNode.Required, otherNode.Required)

	// Simple boolean properties
	warnIfBothSetAndAreDifferent(metadata, "uniqueItems", baseNode.UniqueItems, otherNode.UniqueItems)
	baseNode.UniqueItems = util.GetPtr(otherNode.UniqueItems, baseNode.UniqueItems)

	return baseNode
}

func warnIfBothSetAndAreDifferent[T comparable](metadata *parserMetadata, field string, a *T, b *T) {
	if a != nil && b != nil && *a != *b {
		warnConfigMergeError(metadata, field, fmt.Errorf("both values set during merge (%v vs %v)", *a, *b))
	}
}

func warnConfigMergeError(metadata *parserMetadata, field string, err error) {
	metadata.Errors.AddErrorWithSubpath("/config_merge_error", fmt.Errorf("error merging field %s: %w", field, err))
}

func mergeResolveReference(metadata *parserMetadata, node schemaNode) (schemaNode, error) {
	var err error
	resolvedPaths := []string{}
	refNode := &node

	defer (func() {
		metadata.DocumentResolver.SetDocumentBeingResolved(metadata.DocumentResolver.GetDocumentIdCurrentlyBeingParsed())
	})()

	for refNode.Ref != nil {
		var subRefPath string
		ref := util.GetZeroIfNil(refNode.Ref, "")
		refNode, subRefPath, err = metadata.DocumentResolver.ResolvePath(metadata, ref)

		if err != nil {
			errPath := fmt.Sprintf("/config_ref_merge_error[%s]", subRefPath)
			formattedErr := fmt.Errorf("failed to resolve ref [%s] Error given: %w", subRefPath, err)
			metadata.Errors.AddErrorWithSubpath(errPath, formattedErr)
			return schemaNode{}, formattedErr
		}

		if refNode == nil {
			errPath := fmt.Sprintf("/config_ref_merge_error[%s]", subRefPath)
			formattedErr := fmt.Errorf("failed to resolve ref [%s] Error given: resolved to nil", subRefPath)
			metadata.Errors.AddErrorWithSubpath(errPath, formattedErr)
			return schemaNode{}, formattedErr
		}

		if funk.Contains(resolvedPaths, subRefPath) {
			err = fmt.Errorf("circular reference detected while building composition element reference path %s", subRefPath)
			metadata.Errors.AddErrorWithSubpath("/$ref", err)
			return schemaNode{}, err
		}
		resolvedPaths = append(resolvedPaths, subRefPath)
	}

	// Rewrite references relative to the document we are currently parsing so they remain valid when the parent scope changes post-merge
	newNode, err := metadata.DocumentResolver.RewriteReferencesRelativeToDocument(*refNode)
	if err != nil {
		errPath := "/config_ref_rewrite_error"
		formattedErr := fmt.Errorf("failed to rewrite references for ref [%s]: %e", *node.Ref, err)
		metadata.Errors.AddErrorWithSubpath(errPath, formattedErr)
		return schemaNode{}, formattedErr
	}

	return newNode, nil
}

func mergeSchemaTypes(mergedSchemaType *multipleType, nodeType *multipleType) *multipleType {
	if nodeType == nil {
		return mergedSchemaType
	}

	mergedType := util.GetZeroIfNil(mergedSchemaType, multipleType{})

	if nodeType.SingleType != "" {
		mergedType.SingleType = nodeType.SingleType
		mergedType.MultipleTypes = nil
	} else if len(nodeType.MultipleTypes) > 0 {
		mergedType.MultipleTypes = nodeType.MultipleTypes
		mergedType.SingleType = ""
	}

	return &mergedType
}

// Merges items data (Legacy support for "items" merging not supported)
func mergeItemsData(parserMetadata *parserMetadata, mergedData *itemsData, nodeData *itemsData) *itemsData {
	if nodeData == nil || nodeData.Node == nil {
		return mergedData
	}

	mergedNodeData := util.GetZeroIfNil(mergedData, itemsData{})
	if mergedNodeData.DisallowAdditionalItems || nodeData.DisallowAdditionalItems {
		// If we are disallowing additional items disregard any other data
		return &itemsData{
			DisallowAdditionalItems: true,
		}
	}

	if mergedNodeData.Node != nil && nodeData.Node != nil {
		node, err := mergeSchemaNodes(parserMetadata, *mergedNodeData.Node, *nodeData.Node)
		if err != nil {
			warnConfigMergeError(parserMetadata, "items", err)
		} else {
			mergedNodeData.Node = &node
		}
	} else {
		mergedNodeData.Node = nodeData.Node
	}

	return &mergedNodeData
}

func mergeNodeOrFalse(metadata *parserMetadata, mergedNode *schemaNodeOrFalse, node *schemaNodeOrFalse, field string) *schemaNodeOrFalse {
	if node == nil {
		return mergedNode
	}

	if mergedNode == nil {
		return node
	}

	if mergedNode.IsFalse {
		return mergedNode
	}

	if node.IsFalse {
		return node
	}

	if mergedNode.Schema == nil {
		return node
	}

	if node.Schema == nil {
		return mergedNode
	}

	mergedSchemaNode, err := mergeSchemaNodes(metadata, *mergedNode.Schema, *node.Schema)
	if err != nil {
		warnConfigMergeError(metadata, field, err)
		return mergedNode
	}

	return &schemaNodeOrFalse{
		Schema: &mergedSchemaNode,
	}
}

func mergeIf(metadata *parserMetadata, mergedNode schemaNode, node schemaNode) schemaNode {

	// Propagate any already-merged if statements from the source node.
	// This is critical when a node that went through allOf merge (which populates
	// mergedIf) is later re-merged by parseNot or other code paths — without
	// this, the if/then/else constraints from allOf branches would be silently lost.
	if len(node.mergedIf) > 0 {
		mergedNode.mergedIf = append(mergedNode.mergedIf, node.mergedIf...)
	}

	if node.If != nil {
		path := fmt.Sprintf("%s/if", metadata.ReferenceHandler.CurrentPath)
		mergedNode.mergedIf = append(mergedNode.mergedIf, NewIfStatement(node, path))
	}

	return mergedNode
}

func mergeSchemaPtrs(metadata *parserMetadata, field string, nodes ...*schemaNode) *schemaNode {
	nonNilNodes := []schemaNode{}
	for _, node := range nodes {
		if node != nil {
			nonNilNodes = append(nonNilNodes, *node)
		}
	}

	if len(nonNilNodes) == 0 {
		return nil
	} else if len(nonNilNodes) == 1 {
		return &nonNilNodes[0]
	}

	mergedNode, err := mergeSchemaNodes(metadata, nonNilNodes...)
	if err != nil {
		warnConfigMergeError(metadata, field, err)
		return nil
	}

	return &mergedNode
}

// intersectEnumSlices computes the set intersection of two enum value slices.
// Values are compared by their JSON serialization so that objects and arrays
// are handled correctly (same approach used by notApplyEnum / notApplyConst).
func intersectEnumSlices(a *[]interface{}, b *[]interface{}) *[]interface{} {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	bSet := make(map[string]struct{}, len(*b))
	for _, v := range *b {
		bSet[util.MarshalJsonToString(v)] = struct{}{}
	}

	var result []interface{}
	for _, v := range *a {
		if _, ok := bSet[util.MarshalJsonToString(v)]; ok {
			result = append(result, v)
		}
	}

	return &result
}
