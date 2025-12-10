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
		// Resolve references
		resolvedReference := false

		for node.Ref != nil {
			documentId, path, err := metadata.DocumentResolver.ResolveDocumentIdAndPath(*node.Ref)

			if err != nil {
				warnConfigMergeError(metadata, "/$ref", fmt.Errorf("failed to resolve document for ref [%s]: %w", *node.Ref, err))
				node.Ref = nil
				break
			}
			// Give up on the node if it is a circular reference
			// We cannot easily resolve partial cases for this (especially for factoring or similar)
			if metadata.ReferenceResolver.HasResolved(documentId, path) {
				node.Ref = nil
				break
			}

			finalRefPath := fmt.Sprintf("%s%s", documentId, path)
			node.Ref = &finalRefPath

			refNode, err := mergeResolveReference(metadata, node)
			if err != nil {
				warnConfigMergeError(metadata, "/$ref", fmt.Errorf("failed to resolve ref [%s]: %w", *node.Ref, err))
				node.Ref = nil
				break
			}

			metadata.ReferenceResolver.PushRefResolution(documentId, *node.Ref)
			resolvedReference = true
			node = refNode
		}

		// Merge Type
		mergedNode.Type = mergeSchemaTypes(mergedNode.Type, node.Type)

		if node.Enum != nil {
			mergedNode.Enum = util.MergeSlicePtrs(mergedNode.Enum, node.Enum)
		}

		if node.Const != nil {
			nodeConstValue := util.MarshalJsonToString(node.Const)
			mergedNodeConstValue := util.MarshalJsonToString(mergedNode.Const)

			if mergedNode.Const != nil && nodeConstValue != mergedNodeConstValue {
				warnConfigMergeError(metadata, "const", fmt.Errorf("conflicting const values during schema merge (%s != %s)", mergedNodeConstValue, nodeConstValue))
			} else {
				mergedNode.Const = node.Const
			}
		}

		if node.Properties != nil {
			nodeProperties := util.GetZeroIfNil(node.Properties, map[string]schemaNode{})
			mergedProperties := util.GetZeroIfNil(mergedNode.Properties, map[string]schemaNode{})

			for key, value := range nodeProperties {
				node, err := mergeSchemaNodes(metadata, mergedProperties[key], value)
				if err != nil {
					errPath := fmt.Sprintf("/properties/%s/config_merge_error", key)
					metadata.Errors.AddErrorWithSubpath(errPath, err)

					warnConfigMergeError(metadata, fmt.Sprintf("properties/%s", key), err)
				}
				mergedProperties[key] = node
			}

			mergedNode.Properties = &mergedProperties
		}
		mergedNode.AdditionalProperties = mergeNodeOrFalse(metadata, mergedNode.AdditionalProperties, node.AdditionalProperties, "additionalProperties")

		if node.PatternProperties != nil {
			nodePatternProperties := util.GetZeroIfNil(node.PatternProperties, map[string]schemaNode{})
			mergedPatternProperties := util.GetZeroIfNil(mergedNode.PatternProperties, map[string]schemaNode{})
			for key, value := range nodePatternProperties {
				node, err := mergeSchemaNodes(metadata, mergedPatternProperties[key], value)
				if err != nil {
					warnConfigMergeError(metadata, fmt.Sprintf("patternProperties/%s", key), err)
				}
				mergedPatternProperties[key] = node
			}

			mergedNode.PatternProperties = &mergedPatternProperties
		}

		// Merge array items - @todo: Is this how the schema spec works?
		//                            for merging prefixItems?
		if node.PrefixItems != nil {
			prefixItems := util.GetZeroIfNil(node.PrefixItems, []schemaNode{})
			mergedPrefixItems := util.GetZeroIfNil(mergedNode.PrefixItems, []schemaNode{})
			length := int(math.Max(float64(len(prefixItems)), float64(len(mergedPrefixItems))))
			newMergedPrefixItems := make([]schemaNode, length)
			for i := 0; i < length; i++ {
				node, err := mergeSchemaNodes(metadata,
					util.GetIndexOrDefault(mergedPrefixItems, i, schemaNode{}),
					util.GetIndexOrDefault(prefixItems, i, schemaNode{}),
				)
				if err != nil {
					warnConfigMergeError(metadata, fmt.Sprintf("prefixItems/%d", i), err)
				}
				newMergedPrefixItems[i] = node
			}

			mergedNode.PrefixItems = &newMergedPrefixItems
		}

		// Merge items data
		mergedNode.Items = mergeItemsData(metadata, mergedNode.Items, node.Items)
		mergedNode.AdditionalItems = mergeNodeOrFalse(metadata, mergedNode.AdditionalItems, node.AdditionalItems, "additionalItems")
		mergedNode.UnevaluatedItems = mergeNodeOrFalse(metadata, mergedNode.UnevaluatedItems, node.UnevaluatedItems, "unevaluatedItems")
		mergedNode.Contains = mergeSchemaPtrs(metadata, "contains", mergedNode.Contains, node.Contains)

		mergedNode.OneOf = util.MergeSlicePtrs(mergedNode.OneOf, node.OneOf)
		mergedNode.AnyOf = util.MergeSlicePtrs(mergedNode.AnyOf, node.AnyOf)
		mergedNode.AllOf = util.MergeSlicePtrs(mergedNode.AllOf, node.AllOf)

		// Merge Not
		mergeSchemaPtrs(metadata, "not", mergedNode.Not, node.Not)

		// Merge if / then / else
		mergedNode = mergeIf(metadata, mergedNode, node)

		// Merge simple properties
		mergedNode = mergeSchemaNodeSimpleProperties(metadata, mergedNode, node)

		if resolvedReference {
			// Pop the reference resolution and set the document being resolved back to the previous one
			metadata.ReferenceResolver.PopRefResolution()
			document, _ := metadata.ReferenceResolver.GetCurrentResolution()
			metadata.DocumentResolver.SetDocumentBeingResolved(document)
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
	baseNode.MultipleOf = util.FindHcf(otherNode.MultipleOf, baseNode.MultipleOf)

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
