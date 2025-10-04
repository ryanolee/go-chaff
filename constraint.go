package chaff

import (
	"fmt"
	"regexp"

	"github.com/ryanolee/go-chaff/internal/jsonschema"
	"github.com/ryanolee/go-chaff/internal/util"
	jsonschemaV6 "github.com/santhosh-tekuri/jsonschema/v6"
)

// Constraints applied to post generation of a schema node
type (
	constraintFunction func(value interface{}) bool
	multiConstraint    struct {
		functions map[string]constraintFunction
	}

	constrainedGenerator struct {
		internalGenerator Generator

		// A list of internally applied constraints that require knowing the generated value
		// before making modifications to the output
		constraints []constraint
	}

	oneOfConstraint struct {
		schemas []*jsonschemaV6.Schema
	}

	// Collection of constraints that can be applied at
	constraintCollection struct {
		// Mapping of pattern -> compiled regex
		notMatchingRegexConstraints map[string]regexp.Regexp

		// Mapping of format -> function to validate the format
		notMatchingFormatConstraints map[string]func(any) bool

		// Set of JSON stringified values that the generated value must not be equal to
		notValueConstraints map[string]struct{}
	}

	constraint interface {
		Apply(generator Generator, generatorOptions *GeneratorOptions, generatedValue interface{}) interface{}
		String() string
	}
)

func newConstraintCollection() constraintCollection {
	return constraintCollection{
		notMatchingFormatConstraints: make(map[string]func(any) bool),
		notMatchingRegexConstraints:  make(map[string]regexp.Regexp),
		notValueConstraints:          make(map[string]struct{}),
	}
}

func (cc *constraintCollection) AddNotValueConstraint(values []string) {
	for _, value := range values {
		if _, exists := cc.notValueConstraints[value]; exists {
			continue
		}

		cc.notValueConstraints[value] = struct{}{}
	}
}

func (cc *constraintCollection) AddNotMatchingRegexConstraint(pattern string) error {
	if _, exists := cc.notMatchingRegexConstraints[pattern]; exists {
		return nil
	}

	regexp, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %s", pattern)
	}

	cc.notMatchingRegexConstraints[pattern] = *regexp
	return nil
}

func (cc *constraintCollection) AddNotMatchingFormatConstraint(format string) error {
	if _, exists := cc.notMatchingFormatConstraints[format]; exists {
		return nil
	}

	formatFunc, ok := jsonschema.FormatValidators[format]
	if !ok {
		return fmt.Errorf("unknown format: %s", format)
	}

	cc.notMatchingFormatConstraints[format] = formatFunc
	return nil
}

func (cc *constraintCollection) String() string {
	return fmt.Sprintf("ConstraintCollection[Formats: %s Regexes: %s Values: %s]",
		util.ImplodeMapStrings(cc.notMatchingFormatConstraints),
		util.ImplodeMapStrings(cc.notMatchingRegexConstraints),
		util.ImplodeMapStrings(cc.notValueConstraints),
	)
}

func (mc *constraintCollection) Compile() *multiConstraint {
	constraintFunctions := make(map[string]constraintFunction)

	for pattern, regex := range mc.notMatchingRegexConstraints {
		constraintFunctions[fmt.Sprintf("Regex: %s", pattern)] = func(value interface{}) bool {
			strValue, ok := value.(string)
			if !ok {
				return true
			}
			return !regex.MatchString(strValue)
		}
	}

	for format, formatFunc := range mc.notMatchingFormatConstraints {
		constraintFunctions[fmt.Sprintf("Format: %s", format)] = func(value interface{}) bool {
			return !formatFunc(value)
		}
	}

	if len(mc.notValueConstraints) > 0 {
		constraintFunctions[fmt.Sprintf("NotValues: %s", util.ImplodeMapStrings(mc.notValueConstraints))] = func(value interface{}) bool {
			strValue := util.MarshalJsonToString(value)
			_, exists := mc.notValueConstraints[strValue]
			return !exists
		}
	}

	return &multiConstraint{functions: constraintFunctions}
}

func (mc *multiConstraint) Apply(generator Generator, generatorOptions *GeneratorOptions, generatedValue interface{}) interface{} {
	for i := 0; i < generatorOptions.MaximumUniqueGeneratorAttempts; i++ {
		if mc.constraintPassed(generatedValue) {
			return generatedValue
		}

		generatedValue = generator.Generate(generatorOptions)
	}

	return fmt.Sprintf("Failed to generate a valid value for the following constraints {%s} after %d attempts", mc, generatorOptions.MaximumUniqueGeneratorAttempts)
}

func (mc *multiConstraint) constraintPassed(value interface{}) bool {
	for _, function := range mc.functions {
		if !function(value) {
			return false
		}
	}
	return true
}

func (mc *multiConstraint) String() string {
	return fmt.Sprintf("MultiConstraint{%s}", util.ImplodeMapStrings(mc.functions))
}
