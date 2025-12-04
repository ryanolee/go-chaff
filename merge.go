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

		mergedNode.OneOf = util.MergeSlicePtrs(mergedNode.OneOf, node.OneOf)
		mergedNode.AnyOf = util.MergeSlicePtrs(mergedNode.AnyOf, node.AnyOf)
		mergedNode.AllOf = util.MergeSlicePtrs(mergedNode.AllOf, node.AllOf)

		// Merge Not
		mergeSchemaPtrs(metadata, "not", mergedNode.Not, node.Not)

		// Merge if / then / else
		mergedNode = mergeIf(metadata, mergedNode, node)

		// Merge simple properties
		mergedNode = mergeSchemaNodeSimpleProperties(mergedNode, node)

		if resolvedReference {
			// Pop the reference resolution and set the document being resolved back to the previous one
			metadata.ReferenceResolver.PopRefResolution()
			document, _ := metadata.ReferenceResolver.GetCurrentResolution()
			metadata.DocumentResolver.SetDocumentBeingResolved(document)
		}
	}

	return mergedNode, nil
}

func mergeSchemaNodeSimpleProperties(baseNode schemaNode, otherNode schemaNode) schemaNode {
	// Merge simple int properties
	baseNode.Length = util.GetPtr(otherNode.Length, baseNode.Length)
	baseNode.MinProperties = util.GetPtr(otherNode.MinProperties, baseNode.MinProperties)
	baseNode.MaxProperties = util.GetPtr(otherNode.MaxProperties, baseNode.MaxProperties)
	baseNode.MinItems = util.GetPtr(otherNode.MinItems, baseNode.MinItems)
	baseNode.MaxItems = util.GetPtr(otherNode.MaxItems, baseNode.MaxItems)
	baseNode.MinContains = util.GetPtr(otherNode.MinContains, baseNode.MinContains)
	baseNode.MaxContains = util.GetPtr(otherNode.MaxContains, baseNode.MaxContains)
	baseNode.MinLength = util.GetPtr(otherNode.MinLength, baseNode.MinLength)
	baseNode.MaxLength = util.GetPtr(otherNode.MaxLength, baseNode.MaxLength)

	// Merge simple float properties
	baseNode.Minimum = util.GetPtr(otherNode.Minimum, baseNode.Minimum)
	baseNode.Maximum = util.GetPtr(otherNode.Maximum, baseNode.Maximum)
	baseNode.ExclusiveMinimum = util.GetPtr(otherNode.ExclusiveMinimum, baseNode.ExclusiveMinimum)
	baseNode.ExclusiveMaximum = util.GetPtr(otherNode.ExclusiveMaximum, baseNode.ExclusiveMaximum)
	baseNode.MultipleOf = util.GetPtr(otherNode.MultipleOf, baseNode.MultipleOf)

	// Merge simple string properties
	baseNode.Pattern = util.GetPtr(otherNode.Pattern, baseNode.Pattern)
	baseNode.Format = util.GetPtr(otherNode.Format, baseNode.Format)

	return baseNode
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

func mergeItemsData(parserMetadata *parserMetadata, mergedData *itemsData, nodeData *itemsData) *itemsData {
	if nodeData == nil || nodeData.Node == nil {
		return mergedData
	}

	mergedNode := util.GetZeroIfNil(mergedData, itemsData{})
	if mergedNode.Node != nil && nodeData.Node != nil {
		node, err := mergeSchemaNodes(parserMetadata, *mergedNode.Node, *nodeData.Node)
		if err != nil {
			warnConfigMergeError(parserMetadata, "items", err)
		} else {
			mergedNode.Node = &node
		}
	} else {
		mergedNode.Node = nodeData.Node
	}

	return &mergedNode
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
