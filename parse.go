package chaff

import (
	"encoding/json"
	"os"
	"regexp/syntax"

	"github.com/ryanolee/go-chaff/internal/regen"
	"github.com/ryanolee/go-chaff/internal/util"
)

type (
	// Options to take into account when parsing a json schema
	ParserOptions struct {
		// Options for the regex generator used for generating strings with the "pattern property"
		RegexStringOptions *regen.GeneratorArgs

		// Options for the regex generator used for pattern properties
		RegexPatternPropertyOptions *regen.GeneratorArgs
	}

	// Struct containing metadata for parse operations within the JSON Schema
	parserMetadata struct {
		// Used to keep track of every referenceable route
		ReferenceHandler *referenceHandler
		ParserOptions    ParserOptions
		Errors           map[string]error

		// Generators that need to have their structures Re-Parsed once all references have been resolved
		ReferenceResolver referenceResolver
		RootNode          schemaNode

		// Global merge depth for the schema
		MergeDepth int

		// Schema management for compiling schemas for internal value validation (Required for where subschemas need to be matched for random value generation)
		SchemaManager *schemaManager
	}

	schemaNode struct {
		// Shared Properties
		Type   *multipleType `json:"type,omitempty"`
		Length *int          `json:"length,omitempty"` // Shared by String and Array

		// Object Properties
		Properties           *map[string]schemaNode `json:"properties,omitempty"`
		AdditionalProperties *additionalData        `json:"additionalProperties,omitempty"`
		PatternProperties    *map[string]schemaNode `json:"patternProperties,omitempty"`
		MinProperties        *int                   `json:"minProperties,omitempty"`
		MaxProperties        *int                   `json:"maxProperties,omitempty"`
		Required             *[]string              `json:"required,omitempty"`

		// String Properties
		Pattern   *string `json:"pattern,omitempty"`
		Format    *string `json:"format,omitempty"`
		MinLength *int    `json:"minLength,omitempty"`
		MaxLength *int    `json:"maxLength,omitempty"`

		// Number Properties
		Minimum          *float64 `json:"minimum,omitempty"`
		Maximum          *float64 `json:"maximum,omitempty"`
		ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
		ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
		MultipleOf       *float64 `json:"multipleOf,omitempty"`

		// Array Properties
		Items    *itemsData `json:"items,omitempty"`
		MinItems *int       `json:"minItems,omitempty"` // N Done
		MaxItems *int       `json:"maxItems,omitempty"` // N Done

		Contains    *schemaNode `json:"contains,omitempty"`
		MinContains *int        `json:"minContains,omitempty"` // N Done
		MaxContains *int        `json:"maxContains,omitempty"` // N Done

		PrefixItems      *[]schemaNode      `json:"prefixItems,omitempty"`
		AdditionalItems  *schemaNodeOrFalse `json:"additionalItems,omitempty"`
		UnevaluatedItems *schemaNodeOrFalse `json:"unevaluatedItems,omitempty"`
		UniqueItems      *bool              `json:"uniqueItems,omitempty"`

		// Enum Properties
		Enum *[]interface{} `json:"enum,omitempty"`

		// Constant Properties
		Const *interface{} `json:"const,omitempty"`

		// Combination Properties
		// TODO: Implement these
		Not   *schemaNode   `json:"not,omitempty"`
		AllOf *[]schemaNode `json:"allOf,omitempty"`
		AnyOf *[]schemaNode `json:"anyOf,omitempty"`
		OneOf *[]schemaNode `json:"oneOf,omitempty"`

		// Reference Operator
		Ref         *string                `json:"$ref,omitempty"`
		Id          *string                `json:"$id,omitempty"`
		Defs        *map[string]schemaNode `json:"$defs,omitempty"`
		Definitions *map[string]schemaNode `json:"definitions,omitempty"`

		// Conditional logic
		If   *schemaNode `json:"if,omitempty"`
		Then *schemaNode `json:"then,omitempty"`
		Else *schemaNode `json:"else,omitempty"`

		// Unsupported Properties
		DependentRequired map[string][]string   `json:"dependentRequired,omitempty"`
		DependentSchemas  map[string]schemaNode `json:"dependentSchemas,omitempty"`

		// Internal functionality
		// Used to keep track of ifs from allOf statements that have been merged into this node (or factored into said node)
		mergedIf []ifStatement
	}
)

const (
	// Data Type Operations
	typeObject  = "object"
	typeArray   = "array"
	typeNumber  = "number"
	typeInteger = "integer"
	typeString  = "string"
	typeBoolean = "boolean"
	typeNull    = "null"
	typeUnknown = "unknown"
)

var (
	typeAll = []string{
		typeObject, typeArray, typeNumber, typeInteger, typeString, typeBoolean, typeNull,
	}
)

// Parses a Json Schema file at the given path. If there is an error reading the file or
// parsing the schema, an error will be returned
func ParseSchemaFile(path string, opts *ParserOptions) (RootGenerator, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RootGenerator{
			Generator: nullGenerator{},
		}, err
	}

	return ParseSchema(data, opts)
}

// Parses a Json Schema file at the given path with default options. If there is an error reading the file or
// parsing the schema, an error will be returned
func ParseSchemaFileWithDefaults(path string) (RootGenerator, error) {
	return ParseSchemaFile(path, &ParserOptions{})
}

// Parses a Json Schema string. If there is an error parsing the schema, an error will be returned.
func ParseSchemaString(schema string, opts *ParserOptions) (RootGenerator, error) {
	return ParseSchema([]byte(schema), opts)
}

func ParseSchemaStringWithDefaults(schema string) (RootGenerator, error) {
	return ParseSchemaString(schema, &ParserOptions{})
}

// Parses a Json Schema byte array. If there is an error parsing the schema, an error will be returned.
func ParseSchema(schema []byte, opts *ParserOptions) (RootGenerator, error) {
	var node schemaNode
	err := json.Unmarshal(schema, &node)
	if err != nil {
		return RootGenerator{
			Generator: nullGenerator{},
		}, err
	}

	schemaManager, err := newSchemaManager(schema)
	if err != nil {
		return RootGenerator{
			Generator: nullGenerator{},
		}, err
	}

	refHandler := newReferenceHandler()
	metadata := &parserMetadata{
		ReferenceHandler: &refHandler,
		SchemaManager:    schemaManager,
		Errors:           make(map[string]error),
		ParserOptions:    withDefaultParseOptions(*opts),
		RootNode:         node,
		MergeDepth:       0,
	}
	generator, err := parseRoot(node, metadata)

	return generator, err
}

// Parses a Json Schema byte array with default options. If there is an error parsing the schema, an error will be returned.
func ParseSchemaWithDefaults(schema []byte) (RootGenerator, error) {
	return ParseSchema(schema, &ParserOptions{})
}

func parseNode(node schemaNode, metadata *parserMetadata) (Generator, error) {
	refHandler := metadata.ReferenceHandler
	gen, err := parseSchemaNode(node, metadata)

	if err != nil {
		metadata.Errors[refHandler.CurrentPath] = err
	}

	if node.Id != nil {
		refHandler.AddIdReference(*node.Id, node, gen)
	}

	refHandler.AddReference(node, gen)
	return gen, err

}

func parseSchemaNode(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if err := assertNoUnsupported(node); err != nil {
		return nullGenerator{}, err
	}

	// Handle reference nodes
	if node.Ref != nil {
		return parseReference(node, metadata)
	}

	if node.AllOf != nil {
		return parseAllOf(node, metadata)
	}

	// Handle combination nodes
	if node.OneOf != nil || node.AnyOf != nil {
		return parseCombination(node, metadata)
	}

	// Handle not nodes
	if node.Not != nil {
		return parseNot(node, metadata)
	}

	// Handle conditional nodes
	if node.If != nil || len(node.mergedIf) != 0 {
		return parseIf(node, metadata)
	}

	// Handle enum nodes
	if node.Enum != nil && len(*node.Enum) != 0 {
		return parseEnum(node, metadata)
	}

	// Handle constant nodes
	if node.Const != nil {
		return parseConst(node, metadata)
	}

	// Handle multiple type nodes
	if node.Type != nil && node.Type.MultipleTypes != nil {
		return parseMultipleType(node, metadata)
	}

	// In the case an explicit type is given use that type directly
	if node.Type != nil && node.Type.SingleType != "" {
		return parseType(node.Type.SingleType, node, metadata)
	}

	// Attempt to infer the type of node given the passed properties
	inferredNodeType := inferType(node)

	// In no property type is given assume "any" type is valid in the passed case
	if inferredNodeType == typeUnknown {
		node.Type.MultipleTypes = typeAll
		return parseMultipleType(node, metadata)
	}

	return parseType(inferredNodeType, node, metadata)
}

func parseType(nodeType string, node schemaNode, metadata *parserMetadata) (Generator, error) {
	// Handle object nodes
	switch nodeType {
	case typeObject:
		return parseObject(node, metadata)
	case typeArray:
		return parseArray(node, metadata)
	case typeNumber:
		return parseNumber(node, generatorTypeNumber)
	case typeInteger:
		return parseNumber(node, generatorTypeInteger)
	case typeString:
		return parseString(node, metadata)
	case typeBoolean:
		return parseBoolean(node)
	case typeNull:
		return parseNull(node)
	default:
		return nullGenerator{}, nil
	}
}

func withDefaultParseOptions(opts ParserOptions) ParserOptions {
	parseOpts := ParserOptions{
		RegexStringOptions:          opts.RegexStringOptions,
		RegexPatternPropertyOptions: opts.RegexPatternPropertyOptions,
	}

	defaultRegexOpts := &regen.GeneratorArgs{
		MaxUnboundedRepeatCount: 10,
		SuppressRandomBytes:     true,
		Flags:                   syntax.PerlX,
	}

	if opts.RegexStringOptions == nil {
		parseOpts.RegexStringOptions = defaultRegexOpts
	}

	if opts.RegexPatternPropertyOptions == nil {
		parseOpts.RegexPatternPropertyOptions = defaultRegexOpts
	}

	return parseOpts
}

// Infers the type of a schema node based on its properties
func inferType(node schemaNode) string {
	// Object Properties
	if util.AnyNotNil(node.Properties, node.PatternProperties, node.MinProperties, node.MaxProperties, node.Required) ||
		(node.AdditionalProperties != nil && node.AdditionalProperties.Schema != nil) {
		return typeObject
	}

	// String Properties
	if util.AnyNotNil(node.Pattern, node.Format, node.MinLength, node.MaxLength) {
		return typeString
	}

	// Number Properties
	if util.AnyNotNil(node.Minimum, node.Maximum, node.ExclusiveMinimum, node.ExclusiveMaximum, node.MultipleOf) {
		return typeNumber
	}

	// Array Properties
	if util.AnyNotNil(node.Items.Node, node.MinItems, node.MaxItems, node.Contains, node.MinContains, node.MaxContains, node.PrefixItems, node.AdditionalItems, node.UnevaluatedItems) {
		return typeArray
	}

	// If we can't infer the type, default to null
	return typeUnknown
}
