package chaff

import (
	"fmt"
)

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
		//resolvedReference := false
		
		//for node.Ref != "" {
		//	// Give up on the node if it is a circular reference
		//	// We cannot easily resolve partial cases for this (especially for factoring or similar)
		//	if metadata.ReferenceResolver.HasResolved(node.Ref) {
		//		continue	
		//	}
		//	refNode, err := mergeResolveReference(metadata, node)
		//	
		//	if err != nil {
		//		continue
		//	}
		//	metadata.ReferenceResolver.PushRefResolution(node.Ref)
		//	resolvedReference = true
		//	fmt.Println(metadata.ReferenceResolver.resolutions)	
		//	node = refNode		
		//}

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

		// Merge properties
		refHandler := metadata.ReferenceHandler
		for key, value := range node.Properties {
			node, err := mergeSchemaNodes(metadata, mergedNode.Properties[key], value)
			if err != nil {
				errPath := fmt.Sprintf("%s/properties/%s/config_merge_error", refHandler.CurrentPath, key)
				metadata.Errors[errPath] = err
			}
			mergedNode.Properties[key] = node
		}

		for key, value := range node.PatternProperties {
			node, err := mergeSchemaNodes(metadata, mergedNode.PatternProperties[key], value)
			if err != nil {
				errPath := fmt.Sprintf("%s/patternProperties/%s/config_merge_error", refHandler.CurrentPath, key)
				metadata.Errors[errPath] = err
			}
			mergedNode.PatternProperties[key] = node
		}

		// Merge array items - @todo: Is this how the schema spec works?
		//                            for merging prefixItems?
		for i := 0; i < len(node.PrefixItems); i++ {
			node, err := mergeSchemaNodes(metadata, mergedNode.PrefixItems[i], node.PrefixItems[i])
			if err != nil {
				errPath := fmt.Sprintf("%s/prefixItems/%d/config_merge_error", refHandler.CurrentPath, i)
				metadata.Errors[errPath] = err
			}
			mergedNode.PrefixItems[i] = node
		}

		mergedNode.OneOf = append(mergedNode.OneOf, node.OneOf...)
		mergedNode.AnyOf = append(mergedNode.AnyOf, node.AnyOf...)
		mergedNode.AllOf = append(mergedNode.AllOf, node.AllOf...)

		mergedNode = mergeSchemaNodeSimpleProperties(mergedNode, node)

		//if resolvedReference {
		//	metadata.ReferenceResolver.PopRefResolution()
		//}
	}

	return mergedNode, nil
}

func mergeSchemaNodeSimpleProperties(baseNode schemaNode, otherNode schemaNode) (schemaNode){
	// Merge simple int properties
	baseNode.Length = getInt(otherNode.Length, baseNode.Length)
	baseNode.MinProperties = getInt(otherNode.MinProperties, baseNode.MinProperties)
	baseNode.MaxProperties = getInt(otherNode.MaxProperties, baseNode.MaxProperties)
	baseNode.MinItems = getInt(otherNode.MinItems, baseNode.MinItems)
	baseNode.MaxItems = getInt(otherNode.MaxItems, baseNode.MaxItems)
	baseNode.MinContains = getInt(otherNode.MinContains, baseNode.MinContains)
	baseNode.MaxContains = getInt(otherNode.MaxContains, baseNode.MaxContains)

	// Merge simple float properties
	baseNode.Minimum = getFloat(otherNode.Minimum, baseNode.Minimum)
	baseNode.Maximum = getFloat(otherNode.Maximum, baseNode.Maximum)
	baseNode.ExclusiveMinimum = getFloat(otherNode.ExclusiveMinimum, baseNode.ExclusiveMinimum)
	baseNode.ExclusiveMaximum = getFloat(otherNode.ExclusiveMaximum, baseNode.ExclusiveMaximum)
	baseNode.MultipleOf = getFloat(otherNode.MultipleOf, baseNode.MultipleOf)

	// Merge simple string properties
	baseNode.Pattern = getString(otherNode.Pattern, baseNode.Pattern)
	baseNode.Format = getString(otherNode.Format, baseNode.Format)

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