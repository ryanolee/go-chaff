package chaff

import (
	"fmt"

	"github.com/thoas/go-funk"
)

// Generator for the "allOf" keyword
type allOfGenerator struct {
	SchemaNodes []schemaNode
	MergedNode  schemaNode
	Generator   Generator
	Processed   bool
}

// Parses the "allOf" keyword
func parseAllOf(node schemaNode, metadata *parserMetadata) (*allOfGenerator, error) {
	generator := &allOfGenerator{
		SchemaNodes: node.AllOf,
	}
	metadata.UnprocessedGenerators = append(metadata.UnprocessedGenerators, generator)
	return generator, nil
}

func (g *allOfGenerator) AfterParse(metadata *parserMetadata) error {
	mergedNode, _ := mergeSchemaNodes(metadata, g.SchemaNodes...)
	g.MergedNode = mergedNode
	generator, err := parseSchemaNode(g.MergedNode, metadata)
	if err != nil {
		g.Generator = NullGenerator{}
	} else {
		g.Generator = generator
	}
	g.Processed = true

	return err
}

func (g *allOfGenerator) Generate(opts *GeneratorOptions) interface{} {
	if !g.Processed {
		return nil
	}

	return g.Generator.Generate(opts)
}

func (g *allOfGenerator) String() string {
	return fmt.Sprintf("AllOfGenerator[%s]", g.Generator)
}

func mergeSchemaNodes(metadata *parserMetadata, nodes ...schemaNode) (schemaNode, error) {
	mergedNode := schemaNode{
		Type:              multipleType{},
		Enum:              make([]interface{}, 0),
		Properties:        map[string]schemaNode{},
		PatternProperties: map[string]schemaNode{},
	}

	for _, node := range nodes {
		resolvedReference := false
		if node.Ref != "" {
			ref, ok := metadata.ReferenceHandler.Lookup(node.Ref)
			if !ok {
				errPath := fmt.Sprintf("%s/config_ref_merge_error[%s]", metadata.ReferenceHandler.CurrentPath, node.Ref)
				metadata.Errors[errPath] = fmt.Errorf("reference not found: %s", node.Ref)
				continue
			}

			metadata.ReferenceResolver.PushRefResolution(ref.Path)

			node = ref.SchemaNode
			resolvedReference = true
		}

		// Merge Type
		if node.Type.SingleType != "" {
			mergedNode.Type.MultipleTypes = funk.UniqString(append(mergedNode.Type.MultipleTypes, node.Type.SingleType))
		} else if len(node.Type.MultipleTypes) > 0 {
			mergedNode.Type.MultipleTypes = funk.UniqString(append(mergedNode.Type.MultipleTypes, node.Type.MultipleTypes...))
		}

		// Merge simple int properties
		mergedNode.Length = getInt(node.Length, mergedNode.Length)
		mergedNode.MinProperties = getInt(node.MinProperties, mergedNode.MinProperties)
		mergedNode.MaxProperties = getInt(node.MaxProperties, mergedNode.MaxProperties)
		mergedNode.MinItems = getInt(node.MinItems, mergedNode.MinItems)
		mergedNode.MaxItems = getInt(node.MaxItems, mergedNode.MaxItems)
		mergedNode.MinContains = getInt(node.MinContains, mergedNode.MinContains)
		mergedNode.MaxContains = getInt(node.MaxContains, mergedNode.MaxContains)

		// Merge simple float properties
		mergedNode.Minimum = getFloat(node.Minimum, mergedNode.Minimum)
		mergedNode.Maximum = getFloat(node.Maximum, mergedNode.Maximum)
		mergedNode.ExclusiveMinimum = getFloat(node.ExclusiveMinimum, mergedNode.ExclusiveMinimum)
		mergedNode.ExclusiveMaximum = getFloat(node.ExclusiveMaximum, mergedNode.ExclusiveMaximum)
		mergedNode.MultipleOf = getFloat(node.MultipleOf, mergedNode.MultipleOf)

		// Merge simple string properties
		mergedNode.Pattern = getString(node.Pattern, mergedNode.Pattern)
		mergedNode.Format = getString(node.Format, mergedNode.Format)

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

		if resolvedReference {
			metadata.ReferenceResolver.PopRefResolution()
		}
	}

	return mergedNode, nil
}
