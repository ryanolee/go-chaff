package chaff

import (
	"fmt"
	"strings"

	"github.com/ryanolee/go-chaff/internal/util"
	jsonschemaV6 "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/thoas/go-funk"
)

type (
	combinationGenerator struct {
		Generators []Generator
		Type       string
	}
)

// Parses the "oneOf" or "anyOf" keyword of a schema. This generator is experimental and may not work as expected.
// Example:
//
//	{
//	  "oneOf": [
//	    { "type": "string" },
//	    { "type": "number" }
//	  ]
//	}
//
// One of has a similar implementation to anyOf, so they are both handled by this function
// There are some edge cases that are not handled by this function, such as:
//   - During "factoring" of the schema merging might not work as expected (Reference resolution is not supported as part of this)
//   - oneOf Does not actually validate that only one of the schemas is valid.
func parseCombination(node schemaNode, metadata *parserMetadata) (Generator, error) {
	ref := metadata.ReferenceHandler
	oneOf := util.GetZeroIfNil(node.OneOf, []schemaNode{})
	anyOf := util.GetZeroIfNil(node.AnyOf, []schemaNode{})

	target := anyOf
	nodeType := "anyOf"
	if len(oneOf) > 0 {
		target = oneOf
		nodeType = "oneOf"

	}

	generators := []Generator{}

	for i, subSchema := range target {
		baseNode, _ := mergeSchemaNodes(metadata, node)
		// Stop infinite recursion during merge
		if nodeType == "oneOf" {
			baseNode.OneOf = nil
		}

		if nodeType == "anyOf" {
			baseNode.AnyOf = nil
		}

		mergedNode, err := mergeSchemaNodes(metadata, baseNode, subSchema)
		if err != nil {
			generators = append(generators, nullGenerator{})
			continue
		}

		refPath := fmt.Sprintf("/%s/%d", nodeType, i)

		generator, err := ref.ParseNodeInScope(refPath, mergedNode, metadata)
		if err != nil {
			metadata.Errors.AddErrorWithSubpath(refPath, fmt.Errorf("failed to parse %s sub-schema: %w", nodeType, err))
			generators = append(generators, nullGenerator{})
		} else {
			generators = append(generators, generator)
		}
	}

	if len(oneOf) > 1 {
		stubNode := newEmptySchemaNode()
		resolvedNodes := []schemaNode{}
		for i, node := range oneOf {
			resolvedNode, err := mergeSchemaNodes(metadata, node)
			if err != nil {
				metadata.Errors.AddErrorWithSubpath(fmt.Sprintf("oneOf/%d", i), fmt.Errorf("failed to resolve oneOf sub-schema: %w", err))
				continue
			}

			resolvedNodes = append(resolvedNodes, resolvedNode)
		}

		stubNode.OneOf = &resolvedNodes
		oneOfConstraint, err := NewOneOfConstraint(stubNode, metadata)
		if err != nil {
			return nullGenerator{}, err
		}

		return constrainedGenerator{
			internalGenerator: combinationGenerator{
				Generators: generators,
				Type:       nodeType,
			},
			constraints: []constraint{oneOfConstraint},
		}, nil
	}

	if len(generators) == 1 {
		return generators[0], nil
	} else if len(generators) == 0 {
		return nullGenerator{}, fmt.Errorf("no valid generators could be created for %s", nodeType)
	}

	return combinationGenerator{
		Generators: generators,
		Type:       nodeType,
	}, nil
}

func (g combinationGenerator) Generate(opts *GeneratorOptions) interface{} {
	// Select a random generator
	generator := g.Generators[opts.Rand.RandomInt(0, len(g.Generators))]
	return generator.Generate(opts)
}

func (g combinationGenerator) String() string {
	formattedGenerators := funk.Map(g.Generators, func(generator Generator) string {
		return generator.String()
	}).([]string)
	return fmt.Sprintf("CombinationGenerator[%s]{%s}", g.Type, strings.Join(formattedGenerators, ","))
}

func NewOneOfConstraint(node schemaNode, metadata *parserMetadata) (*oneOfConstraint, error) {
	schemas := []*jsonschemaV6.Schema{}

	if node.OneOf == nil || len(*node.OneOf) == 0 {
		return nil, fmt.Errorf("no oneOf schemas found")
	}

	for key, node := range *node.OneOf {
		path := fmt.Sprintf("oneOf/%d", key)
		compiledSchema, err := metadata.SchemaManager.ParseSchemaNode(metadata, node, path)

		if err != nil {
			metadata.Errors.AddErrorWithSubpath(path, err)
			continue
		}

		schemas = append(schemas, compiledSchema)
	}

	return &oneOfConstraint{
		schemas: schemas,
	}, nil
}

func (oc *oneOfConstraint) Apply(generator Generator, generatorOptions *GeneratorOptions, generatedValue interface{}) interface{} {
	for i := 0; i < generatorOptions.MaximumOneOfAttempts; i++ {
		generatorOptions.overallComplexity++
		if oc.constraintPassed(generatedValue) {
			return generatedValue
		}

		generatedValue = generator.Generate(generatorOptions)
	}

	return fmt.Sprintf("Failed to generate a valid value for the following oneOf constraint after %d attempts", generatorOptions.MaximumOneOfAttempts)
}

func (oc *oneOfConstraint) constraintPassed(value interface{}) bool {
	matchingSchemas := 0
	for _, schema := range oc.schemas {
		if schema.Validate(value) == nil {
			matchingSchemas++
		}

		if matchingSchemas > 1 {
			return false
		}

	}

	return matchingSchemas == 1
}

func (oc *oneOfConstraint) String() string {
	return fmt.Sprintf("OneOfConstraint[NumSchemas: %d]", len(oc.schemas))
}

func (g constrainedGenerator) Generate(opts *GeneratorOptions) interface{} {
	generatedValue := g.internalGenerator.Generate(opts)
	for _, constraint := range g.constraints {
		opts.overallComplexity++
		generatedValue = constraint.Apply(g.internalGenerator, opts, generatedValue)
	}

	return generatedValue
}

func (g constrainedGenerator) String() string {
	constraintStrings := funk.Map(g.constraints, func(c constraint) string {
		return c.String()
	}).([]string)
	return fmt.Sprintf("ConstrainedGenerator{constraints: %s, internalGenerator: %s}", strings.Join(constraintStrings, ","), g.internalGenerator)
}
