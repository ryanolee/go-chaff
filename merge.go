package chaff

import (
	"fmt"
	"regexp"

	"github.com/ryanolee/go-chaff/internal/util"
)

var mergeRegex = regexp.MustCompile(`\/(allOf|anyOf|oneOf)\/`)

func newEmptySchemaNode() schemaNode {
	return schemaNode{
		Type:              multipleType{},
		Enum:              make([]interface{}, 0),
		Properties:        map[string]schemaNode{},
		PatternProperties: map[string]schemaNode{},
	}
}

// Merges all sub properties of a given node
func mergeSchemaNodes(metadata *parserMetadata, nodes ...schemaNode) (schemaNode, error) {
	mergedNode := newEmptySchemaNode()

	for _, node := range nodes {
		// Resolve references
		resolvedReference := false

		for node.Ref != "" {
			// Give up on the node if it is a circular reference
			// We cannot easily resolve partial cases for this (especially for factoring or similar)
			if metadata.ReferenceResolver.HasResolved(node.Ref) {
				node.Ref = ""
				break
			}

			// If the reference is going into an allOf, we should not resolve it
			if len(mergeRegex.FindAllString(metadata.ReferenceHandler.CurrentPath, -1)) >= 2 {
				errPath := fmt.Sprintf("%s/$ref", metadata.ReferenceHandler.CurrentPath)
				metadata.Errors[errPath] = fmt.Errorf("refusing to resolve reference any more composition elements in reference %s", node.Ref)
				node.Ref = ""
				break
			}

			refNode, err := mergeResolveReference(metadata, node)

			if err != nil {
				node.Ref = ""
				break
			}

			metadata.ReferenceResolver.PushRefResolution(node.Ref)
			resolvedReference = true
			node = refNode
		}

		// Merge Type
		if node.Type.SingleType != "" {
			mergedNode.Type.SingleType = node.Type.SingleType
			mergedNode.Type.MultipleTypes = nil
		} else if len(node.Type.MultipleTypes) > 0 {
			mergedNode.Type.MultipleTypes = node.Type.MultipleTypes
			mergedNode.Type.SingleType = ""
		}

		if len(node.Enum) > 0 {
			mergedNode.Enum = append(mergedNode.Enum, node.Enum...)
		}

		for key, value := range node.Properties {
			node, err := mergeSchemaNodes(metadata, mergedNode.Properties[key], value)
			if err != nil {
				errPath := fmt.Sprintf("%s/properties/%s/config_merge_error", metadata.ReferenceHandler.CurrentPath, key)
				metadata.Errors[errPath] = err
			}
			mergedNode.Properties[key] = node
		}

		for key, value := range node.PatternProperties {
			node, err := mergeSchemaNodes(metadata, mergedNode.PatternProperties[key], value)
			if err != nil {
				errPath := fmt.Sprintf("%s/patternProperties/%s/config_merge_error", metadata.ReferenceHandler.CurrentPath, key)
				metadata.Errors[errPath] = err
			}
			mergedNode.PatternProperties[key] = node
		}

		// Merge array items - @todo: Is this how the schema spec works?
		//                            for merging prefixItems?
		for i := 0; i < len(node.PrefixItems); i++ {
			node, err := mergeSchemaNodes(metadata, mergedNode.PrefixItems[i], node.PrefixItems[i])
			if err != nil {
				errPath := fmt.Sprintf("%s/prefixItems/%d/config_merge_error", metadata.ReferenceHandler.CurrentPath, i)
				metadata.Errors[errPath] = err
			}
			mergedNode.PrefixItems[i] = node
		}

		mergedNode.OneOf = append(mergedNode.OneOf, node.OneOf...)
		mergedNode.AnyOf = append(mergedNode.AnyOf, node.AnyOf...)
		mergedNode.AllOf = append(mergedNode.AllOf, node.AllOf...)

		mergedNode = mergeSchemaNodeSimpleProperties(mergedNode, node)

		if resolvedReference {
			metadata.ReferenceResolver.PopRefResolution()
		}
	}

	return mergedNode, nil
}

func mergeSchemaNodeSimpleProperties(baseNode schemaNode, otherNode schemaNode) schemaNode {
	// Merge simple int properties
	baseNode.Length = util.GetInt(otherNode.Length, baseNode.Length)
	baseNode.MinProperties = util.GetInt(otherNode.MinProperties, baseNode.MinProperties)
	baseNode.MaxProperties = util.GetInt(otherNode.MaxProperties, baseNode.MaxProperties)
	baseNode.MinItems = util.GetInt(otherNode.MinItems, baseNode.MinItems)
	baseNode.MaxItems = util.GetInt(otherNode.MaxItems, baseNode.MaxItems)
	baseNode.MinContains = util.GetInt(otherNode.MinContains, baseNode.MinContains)
	baseNode.MaxContains = util.GetInt(otherNode.MaxContains, baseNode.MaxContains)
	baseNode.MinLength = util.GetInt(otherNode.MinLength, baseNode.MinLength)
	baseNode.MaxLength = util.GetInt(otherNode.MaxLength, baseNode.MaxLength)

	// Merge simple float properties
	baseNode.Minimum = util.GetFloat(otherNode.Minimum, baseNode.Minimum)
	baseNode.Maximum = util.GetFloat(otherNode.Maximum, baseNode.Maximum)
	baseNode.ExclusiveMinimum = util.GetFloat(otherNode.ExclusiveMinimum, baseNode.ExclusiveMinimum)
	baseNode.ExclusiveMaximum = util.GetFloat(otherNode.ExclusiveMaximum, baseNode.ExclusiveMaximum)
	baseNode.MultipleOf = util.GetFloat(otherNode.MultipleOf, baseNode.MultipleOf)

	// Merge simple string properties
	baseNode.Pattern = util.GetString(otherNode.Pattern, baseNode.Pattern)
	baseNode.Format = util.GetString(otherNode.Format, baseNode.Format)

	return baseNode
}

func mergeResolveReference(metadata *parserMetadata, node schemaNode) (schemaNode, error) {
	refNode, err := resolveReferencePath(metadata.RootNode, node.Ref)
	if err != nil {
		errPath := fmt.Sprintf("%s/config_ref_merge_error[%s]", metadata.ReferenceHandler.CurrentPath, node.Ref)
		err := fmt.Errorf("failed to resolve ref [%s] Error given: %e", node.Ref, err)
		metadata.Errors[errPath] = fmt.Errorf("failed to resolve ref [%s] Error given: %e", node.Ref, err)
		return schemaNode{}, err
	}

	return refNode, nil

}
