package chaff

import (
	"encoding/json"
	"os"
	"regexp/syntax"

	"github.com/ryanolee/go-chaff/internal/regen"
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
	}

	schemaNode struct {
		// Shared Properties
		Type   multipleType `json:"type"`
		Length int          `json:"length"` // Shared by String and Array

		// Object Properties
		Properties           map[string]schemaNode `json:"properties"`
		AdditionalProperties additionalData        `json:"additionalProperties"`
		PatternProperties    map[string]schemaNode `json:"patternProperties"`
		MinProperties        int                   `json:"minProperties"`
		MaxProperties        int                   `json:"maxProperties"`
		Required             []string              `json:"required"`

		// String Properties
		Pattern   string `json:"pattern"`
		Format    string `json:"format"`
		MinLength int    `json:"minLength"`
		MaxLength int    `json:"maxLength"`

		// Number Properties
		Minimum          float64 `json:"minimum"`
		Maximum          float64 `json:"maximum"`
		ExclusiveMinimum float64 `json:"exclusiveMinimum"`
		ExclusiveMaximum float64 `json:"exclusiveMaximum"`
		MultipleOf       float64 `json:"multipleOf"`

		// Array Properties
		Items    itemsData `json:"items"`
		MinItems int       `json:"minItems"`
		MaxItems int       `json:"maxItems"`

		Contains    *schemaNode `json:"contains"`
		MinContains int         `json:"minContains"`
		MaxContains int         `json:"maxContains"`

		PrefixItems      []schemaNode       `json:"prefixItems"`
		AdditionalItems  *schemaNodeOrFalse `json:"additionalItems"`
		UnevaluatedItems *schemaNodeOrFalse `json:"unevaluatedItems"`
		UniqueItems      bool               `json:"uniqueItems"`

		// Enum Properties
		Enum []interface{} `json:"enum"`

		// Constant Properties
		Const interface{} `json:"const"`

		// Combination Properties
		// TODO: Implement these
		//Not *SchemaNode `json:"not"`
		AllOf []schemaNode `json:"allOf"`
		AnyOf []schemaNode `json:"anyOf"`
		OneOf []schemaNode `json:"oneOf"`

		// Reference Operator
		Ref         string                `json:"$ref"`
		Id          string                `json:"$id"`
		Defs        map[string]schemaNode `json:"$defs"`
		Definitions map[string]schemaNode `json:"definitions"`

		// Unsupported Properties
		Not               *schemaNode           `json:"not"`
		If                *schemaNode           `json:"if"`
		Then              *schemaNode           `json:"then"`
		Else              *schemaNode           `json:"else"`
		DependentRequired map[string][]string   `json:"dependentRequired"`
		DependentSchemas  map[string]schemaNode `json:"dependentSchemas"`
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

	refHandler := newReferenceHandler()
	metadata := &parserMetadata{
		ReferenceHandler: &refHandler,
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

	if node.Id != "" {
		refHandler.AddIdReference(node.Id, node, gen)
	}

	refHandler.AddReference(node, gen)
	return gen, err

}

func parseSchemaNode(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if err := assertNoUnsupported(node); err != nil {
		return nullGenerator{}, err
	}

	// Handle reference nodes
	if node.Ref != "" {
		return parseReference(node, metadata)
	}

	if node.AllOf != nil {
		return parseAllOf(node, metadata)
	}

	// Handle combination nodes
	if node.OneOf != nil || node.AnyOf != nil {
		return parseCombination(node, metadata)
	}

	// Handle enum nodes
	if len(node.Enum) != 0 {
		return parseEnum(node)
	}

	// Handle constant nodes
	if node.Const != nil {
		return parseConst(node)
	}

	// Handle multiple type nodes
	if node.Type.MultipleTypes != nil {
		return parseMultipleType(node, metadata)
	}

	// In the case no explicit type is given
	// attempt to infer the type from the node properties
	if node.Type.SingleType == "" {
		inferredNodeType := inferType(node)
		return parseType(inferredNodeType, node, metadata)
	}

	return parseType(node.Type.SingleType, node, metadata)
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
	if node.Properties != nil ||
		node.AdditionalProperties.Schema != nil ||
		node.PatternProperties != nil ||
		node.MinProperties != 0 ||
		node.MaxProperties != 0 ||
		node.Required != nil {
		return typeObject
	}

	// String Properties
	if node.Pattern != "" ||
		node.Format != "" ||
		node.MinLength != 0 ||
		node.MaxLength != 0 {
		return typeString
	}

	// Number Properties
	if node.Minimum != 0 ||
		node.Maximum != 0 ||
		node.ExclusiveMinimum != 0 ||
		node.ExclusiveMaximum != 0 ||
		node.MultipleOf != 0 {
		return typeNumber
	}

	// Array Properties
	if node.Items.Node != nil ||
		node.MinItems != 0 ||
		node.MaxItems != 0 ||
		node.Contains != nil ||
		node.MinContains != 0 ||
		node.MaxContains != 0 ||
		node.PrefixItems != nil ||
		node.AdditionalItems != nil ||
		node.UnevaluatedItems != nil {
		return typeArray
	}

	// If we can't infer the type, default to null
	return typeNull
}
