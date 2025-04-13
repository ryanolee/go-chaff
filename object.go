package chaff

import (
	"fmt"

	"github.com/ryanolee/go-chaff/internal/regen"
	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/thoas/go-funk"
)

type (
	objectGenerator struct {
		Properties map[string]Generator

		// Pattern Properties Regex -> Generator mapping
		PatternProperties      map[string]Generator
		PatternPropertiesRegex map[string]regen.Generator

		DisallowAdditionalProperties bool
		AdditionalProperties         Generator

		FallbackGenerator Generator

		MinProperties int
		MaxProperties int
		Required      []string
	}
)

// Parses the "type" keyword of a schema when it is an object
// Example:
//
//	{
//	  "type": "object",
//	  "properties": {
//	    "foo": {
//	      "type": "string"
//	    }
//	  },
//	  "required": ["foo"]
//	}
func parseObject(node schemaNode, metadata *parserMetadata) (Generator, error) {
	// Validator Max and Min Properties
	if node.MinProperties < 0 {
		return nullGenerator{}, fmt.Errorf("minProperties must be greater than or equal to 0")
	}

	if node.MaxProperties < 0 {
		return nullGenerator{}, fmt.Errorf("maxProperties must be greater than or equal to 0")
	}

	if node.MaxProperties != 0 && node.MinProperties > node.MaxProperties {
		return nullGenerator{}, fmt.Errorf("minProperties (%d) must be less than or equal to MaxProperties (%d)", node.MinProperties, node.MaxProperties)
	}

	// Validate Required Properties
	if node.MaxProperties != 0 && len(node.Required) > node.MaxProperties {
		return nullGenerator{}, fmt.Errorf("required properties must have a length of less than or equal to MaxProperties (Max Properties: %d, Length of required %d)", node.MaxProperties, len(node.Required))
	}

	// Validate additionalProperties
	if node.AdditionalProperties.DisallowAdditional && node.PatternProperties == nil && node.MinProperties > len(node.Properties) {
		return nullGenerator{}, fmt.Errorf("given additional properties are not allowed and there are no pattern properties the minProperties must be less than or equal to the number of"+
			"available properties. (minProperties: %d, propertiesDefined: %d)", node.MinProperties, len(node.Properties))
	}

	patternProperties, patternPropertiesRegex := parsePatternProperties(node, metadata)

	objectGenerator := objectGenerator{
		Required:      node.Required,
		MinProperties: node.MinProperties,
		MaxProperties: node.MaxProperties,

		Properties:             parseProperties(node, metadata),
		PatternProperties:      patternProperties,
		PatternPropertiesRegex: patternPropertiesRegex,

		DisallowAdditionalProperties: node.AdditionalProperties.DisallowAdditional,
		AdditionalProperties:         parseAdditionalProperties(node, metadata),
		FallbackGenerator:            nullGenerator{},
	}

	return objectGenerator, nil
}

func parseProperties(node schemaNode, metadata *parserMetadata) map[string]Generator {
	properties := make(map[string]Generator)
	ref := metadata.ReferenceHandler
	for name, prop := range node.Properties {
		refPath := fmt.Sprintf("/properties/%s", name)
		propGenerator, err := ref.ParseNodeInScope(refPath, prop, metadata)
		if err != nil {
			propGenerator = nullGenerator{}
		}

		properties[name] = propGenerator
	}

	return properties
}

func parseAdditionalProperties(node schemaNode, metadata *parserMetadata) Generator {
	if node.AdditionalProperties.DisallowAdditional || node.AdditionalProperties.Schema == nil {
		return nil
	}
	ref := metadata.ReferenceHandler
	refPath := "/additionalProperties"
	additionalProperties, err := ref.ParseNodeInScope(refPath, *node.AdditionalProperties.Schema, metadata)

	if err != nil {
		return nullGenerator{}
	}

	return additionalProperties
}

func parsePatternProperties(node schemaNode, metadata *parserMetadata) (map[string]Generator, map[string]regen.Generator) {
	if node.PatternProperties == nil {
		return nil, nil
	}

	propertiesRegex := make(map[string]regen.Generator)
	properties := make(map[string]Generator)
	ref := metadata.ReferenceHandler

	for regex, property := range node.PatternProperties {
		refPath := fmt.Sprintf("/patternProperties/%s", regex)

		// Parse the schema node
		propGenerator, err := ref.ParseNodeInScope(refPath, property, metadata)
		if err != nil {
			propGenerator = nullGenerator{}
		}

		regexGenerator, err := newRegexGenerator(regex, metadata.ParserOptions.RegexPatternPropertyOptions)
		if err != nil {
			errPath := fmt.Sprintf("%s/regex/%s", ref.CurrentPath, regex)
			metadata.Errors[errPath] = fmt.Errorf("failed to create regex generator for %s. Error given: %s", regex, err)
			regexGenerator = nil
		}

		propertiesRegex[regex] = regexGenerator
		properties[regex] = propGenerator
	}

	return properties, propertiesRegex
}

func (g objectGenerator) Generate(opts *GeneratorOptions) interface{} {
	// Handle complexity
	opts.overallComplexity++

	// Generate Required Properties
	generatedValues := make(map[string]interface{})
	for _, key := range g.Required {
		// If no properties are defined, generate a nil value
		if g.Properties == nil {
			generatedValues[key] = fmt.Sprintf("required_%s_%d", key, opts.Rand.RandomInt(0, 9999999))
		} else if _, ok := g.Properties[key]; !ok {
			generatedValues[key] = fmt.Sprintf("required_%s_%d", key, opts.Rand.RandomInt(0, 9999999))
		} else {
			// Generate the required property
			generatedValues[key] = g.Properties[key].Generate(opts)
		}
	}

	// Generate A random distribution of optional properties, pattern properties, and additional properties
	// (Using a fallback generator if none are available)
	optionalKeys := funk.UniqString(append(g.Required, funk.Keys(g.Properties).([]string)...))

	min := util.GetInt(g.MinProperties, opts.DefaultObjectMinProperties)
	max := util.GetInt(g.MaxProperties, opts.DefaultObjectMaxProperties)

	// Make sure the max is always greater than the min
	if max < min {
		max = min + max
	}

	minimumExtrasToGenerate := util.MaxInt(0, min-len(g.Required))
	maximumExtrasToGenerate := util.MaxInt(0, max-len(g.Required))

	generatorTarget := opts.Rand.RandomInt(minimumExtrasToGenerate, maximumExtrasToGenerate)

	if opts.MaximumGenerationSteps > 0 && opts.overallComplexity > opts.MaximumGenerationSteps {
		generatorTarget = minimumExtrasToGenerate
	}

	numberOfOptionalKeysToGenerate := util.MinInt(len(optionalKeys), generatorTarget)
	optionalKeysToGenerate := opts.Rand.StringChoiceMultiple(&optionalKeys, numberOfOptionalKeysToGenerate)

	// Generate any optional keys
	for _, key := range optionalKeysToGenerate {
		if g.Properties == nil {
			generatedValues[key] = fmt.Sprintf("optional_%s_%d", key, opts.Rand.RandomInt(0, 9999999))
		} else if _, ok := g.Properties[key]; !ok {
			generatedValues[key] = fmt.Sprintf("optional_%s_%d", key, opts.Rand.RandomInt(0, 9999999))
		} else {
			generatedValues[key] = g.Properties[key].Generate(opts)
		}
	}

	generatorTarget -= len(optionalKeysToGenerate)

	// Generate any pattern properties
	// Failing that generate any additional properties
	// Failing that generate any fallback properties
	if len(g.PatternProperties) > 0 {
		for i := 0; i < generatorTarget; i++ {
			regex, value := g.GeneratePatternProperty(opts)
			generatedValues[regex] = value
		}
	} else if g.DisallowAdditionalProperties {
		return generatedValues
	} else if g.AdditionalProperties != nil {
		for i := 0; i < generatorTarget; i++ {
			generatedValues[fmt.Sprintf("additional_%d", i)] = g.AdditionalProperties.Generate(opts)
		}
	} else {
		for i := 0; i < generatorTarget; i++ {
			if opts.SuppressFallbackValues || min > len(generatedValues) {
				continue
			}

			generatedValues[fmt.Sprintf("fallback_%d", i)] = g.FallbackGenerator.Generate(opts)
		}
	}

	// In the event the number of generated parameters due to config options
	// results in fewer than the minimum number of properties being generated
	// generate atleast the minimum number of properties required for satisfiability
	if len(generatedValues) < min {
		generator := g.FallbackGenerator
		if g.AdditionalProperties != nil {
			generator = g.AdditionalProperties
		}

		for i := len(generatedValues); i < min; i++ {
			generatedValues[fmt.Sprintf("min_filler_%d", i)] = generator.Generate(opts)
		}

	}

	return generatedValues
}

func (g objectGenerator) GeneratePatternProperty(opts *GeneratorOptions) (string, interface{}) {
	if len(g.PatternProperties) == 0 {
		return "", nil
	}

	availableRegexes := funk.Keys(g.PatternProperties).([]string)
	targetRegex := opts.Rand.StringChoice(&availableRegexes)
	targetRegexGenerator := g.PatternPropertiesRegex[targetRegex]
	targetGenerator := g.PatternProperties[targetRegex]

	if targetGenerator == nil || targetRegexGenerator == nil {
		return "", nil
	}

	return targetRegexGenerator.Generate(), targetGenerator.Generate(opts)
}

func (g objectGenerator) String() string {
	formattedString := ""
	for name, prop := range g.Properties {
		formattedString += fmt.Sprintf("%s: %s,", name, prop)
	}

	regexString := ""
	for regex, prop := range g.PatternProperties {
		regexString += fmt.Sprintf("%s: %s,", regex, prop)
	}

	return fmt.Sprintf("ObjectGenerator{properties: %s, patternProperties: %s, additionalProperties: %s}", formattedString, regexString, g.AdditionalProperties)
}
