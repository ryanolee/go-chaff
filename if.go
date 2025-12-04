package chaff

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thoas/go-funk"
)

type (
	ifConstraint struct {
		conditionFunc func(value any) bool
		thenGenerator Generator
		elseGenerator Generator
	}

	multipleIfConstraints struct {
		constraints []ifConstraint
	}

	ifStatement struct {
		OriginalPath string
		If           *schemaNode
		Then         *schemaNode
		Else         *schemaNode
	}
)

// Parses the "if" keyword of a schema
// Example:
//
//	{
//	  "if": { "properties": { "foo": { "const": "bar" } }, "required": ["foo"] },
//	  "then": { "properties": { "bar": { "const": "baz" } }, "required": ["bar"] },
//	  "else": { "properties": { "bar": { "const": "qux" } }, "required": ["bar"] }
//	}
//
// This uses a strategy of merging the parent schema with the "then" and "else" schemas
// then if the "if" condition is met, the "then" schema is used to generate the value and is left-biased merged with the value that matched the if condition to preserve the "true" state of the "if" condition.
// otherwise the "else" schema is used to generate the value.
func parseIf(node schemaNode, metadata *parserMetadata) (Generator, error) {

	if node.If != nil {
		node.mergedIf = append(node.mergedIf, NewIfStatement(node, metadata.ReferenceHandler.CurrentPath))
	}

	//nullify subschemas to avoid infinite recursion during merge
	mergedIf := node.mergedIf
	node.If, node.Then, node.Else, node.mergedIf = nil, nil, nil, nil

	internalGenerator, err := parseSchemaNode(node, metadata)
	if err != nil {
		return nullGenerator{}, err
	}

	if len(mergedIf) == 0 {
		return internalGenerator, nil
	}

	constraints := []ifConstraint{}
	for i, ifStatement := range mergedIf {
		compiled, err := ifStatement.Compile(node, metadata, fmt.Sprintf("/if/%d", i))
		if err != nil {
			path := fmt.Sprintf("if/%d/config_compile_error", i)
			metadata.Errors.AddErrorWithSubpath(path, err)
			continue
		}

		constraints = append(constraints, compiled)
	}

	if len(constraints) == 0 {
		metadata.Errors.AddErrorWithSubpath("/if/config_compile_error", errors.New("no valid if statements could be compiled"))
		return internalGenerator, nil
	}

	return constrainedGenerator{
		internalGenerator: internalGenerator,
		constraints:       []constraint{multipleIfConstraints{constraints: constraints}},
	}, nil

}

func parseIfBody(metadata *parserMetadata, field string, parentScope schemaNode, bodyNode *schemaNode) Generator {
	if bodyNode == nil {
		return nil
	}

	mergedNode, err := mergeSchemaNodes(metadata, parentScope, *bodyNode)
	if err != nil {

		warnField(metadata, field, fmt.Errorf("failed to merge %s schema node: %w", field, err))
		return nil
	}

	generator, err := metadata.ReferenceHandler.ParseNodeInScope(field, mergedNode, metadata)
	if err != nil {
		warnField(metadata, field, err)
		return nil
	}

	return generator
}

// Internal if statement used to apply the if then then else logic
func NewIfStatement(node schemaNode, nodePath string) ifStatement {
	return ifStatement{
		OriginalPath: nodePath,
		If:           node.If,
		Then:         node.Then,
		Else:         node.Else,
	}
}

func (s ifStatement) Compile(node schemaNode, metadata *parserMetadata, field string) (ifConstraint, error) {
	if s.If == nil {
		return ifConstraint{}, fmt.Errorf("if schema must have an if clause")
	}

	if s.Then == nil && s.Else == nil {
		return ifConstraint{}, fmt.Errorf("if schema must have either then or else")
	}

	ifSchema, err := metadata.SchemaManager.ParseSchemaNode(metadata, *s.If, fmt.Sprintf("%s/%s", metadata.ReferenceHandler.CurrentPath, field))
	if err != nil {
		return ifConstraint{}, fmt.Errorf("failed to compile if sub schema: %w", err)
	}

	thenGenerator := parseIfBody(metadata, "/then", node, s.Then)
	elseGenerator := parseIfBody(metadata, "/else", node, s.Else)

	return ifConstraint{
		conditionFunc: func(value any) bool {
			return ifSchema.Validate(value) == nil
		},
		thenGenerator: thenGenerator,
		elseGenerator: elseGenerator,
	}, nil
}

// Attempt to satisfy the if constraint by shoving the generated value through the then subschma
// and attempting to satisfy the clause again with the subsequent value.
// Returns (value, true) if the constraint was satisfied
// Returns (nil, false) if the constraint could not be satisfied
func (g ifConstraint) AttemptToSatisfyIfStatement(generatorOptions *GeneratorOptions, generatedValue interface{}, mustExactlySatisfy bool) (interface{}, bool) {
	if g.conditionFunc(generatedValue) {
		if g.thenGenerator == nil {
			if mustExactlySatisfy {
				return nil, false
			}
			return generatedValue, true
		}

		thenValue := g.thenGenerator.Generate(generatorOptions)
		if g.conditionFunc(thenValue) {
			return thenValue, true
		}
	} else {
		if g.elseGenerator == nil {
			if mustExactlySatisfy {
				return nil, false
			}
			return generatedValue, true
		}

		elseValue := g.elseGenerator.Generate(generatorOptions)
		if !g.conditionFunc(elseValue) {
			return elseValue, true
		}
	}

	return nil, false
}

// Attempt to satisfy the if constraint by shoving the generated value through the then subschma
// and attempting to satisfy the clause again with the subsequent value.
func (g ifConstraint) Apply(generator Generator, generatorOptions *GeneratorOptions, generatedValue interface{}) interface{} {
	for i := 0; i < generatorOptions.MaximumIfAttempts; i++ {
		generatorOptions.overallComplexity++
		if satisfiedValue, satisfied := g.AttemptToSatisfyIfStatement(generatorOptions, generatedValue, false); satisfied {
			return satisfiedValue
		}

		generatedValue = generator.Generate(generatorOptions)
	}

	return fmt.Sprintf("Failed to generate a valid value for the following if constraint after %d attempts", generatorOptions.MaximumUniqueGeneratorAttempts)
}

func (g ifConstraint) String() string {
	return fmt.Sprintf("IfConstraint[Then: %s Else: %s]", g.thenGenerator, g.elseGenerator)
}

func (g multipleIfConstraints) Apply(generator Generator, generatorOptions *GeneratorOptions, generatedValue interface{}) interface{} {
	// Behavior change if there are multiple if constraints (Where only a direct hit on the then/else will satisfy the constraint)
	mustExactlySatisfy := len(g.constraints) >= 1

	// Brute force against all constraints in random order to try and satisfy at least one
	for i := 0; i < generatorOptions.MaximumIfAttempts; i++ {
		for _, constraintIdx := range generatorOptions.Rand.Rand.Perm(len(g.constraints)) {
			if satisfiedValue, satisfied := g.constraints[constraintIdx].AttemptToSatisfyIfStatement(generatorOptions, generatedValue, mustExactlySatisfy); satisfied {
				return satisfiedValue
			}
		}
	}

	return fmt.Sprintf("Failed to generate a valid value for the following if constraints after %d attempts: [%s]",
		generatorOptions.MaximumIfAttempts,
		g,
	)
}

func (g multipleIfConstraints) String() string {
	return fmt.Sprintf("MultipleIfConstraints[%s]", strings.Join(funk.Map(g.constraints, func(c ifConstraint) string {
		return c.String()
	}).([]string), ", "))
}
