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
		ReferenceResolver     referenceResolver
		RootNode			  schemaNode
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
		Pattern string `json:"pattern"`
		Format  string `json:"format"`

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

		PrefixItems     []schemaNode `json:"prefixItems"`
		AdditionalItems *schemaNode  `json:"additionalItems"`
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
	typeNull   = "null"
)

func ParseSchemaFile(path string, opts *ParserOptions) (rootGenerator, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rootGenerator{
			Generator: NullGenerator{},
		}, err
	}

	return ParseSchema(data, opts)
}

func ParseSchema(schema []byte, opts *ParserOptions) (rootGenerator, error) {
	var node schemaNode
	err := json.Unmarshal(schema, &node)
	if err != nil {
		return rootGenerator{
			Generator: NullGenerator{},
		}, err
	}

	refHandler := newReferenceHandler()
	metadata := &parserMetadata{
		ReferenceHandler: &refHandler,
		Errors:           make(map[string]error),
		ParserOptions:    withDefaultParseOptions(*opts),
		RootNode:		  node,
	}
	generator, err := parseRoot(node, metadata)
	
	return generator, err
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

	return parseType(node.Type.SingleType, node, metadata)
}

func parseType(nodeType string, node schemaNode, metadata *parserMetadata) (Generator, error) {
	// Handle object nodes
	switch nodeType {
	case typeObject:
		return ParseObject(node, metadata)
	case typeArray:
		return parseArray(node, metadata)
	case typeNumber:
		return parseNumber(node, TypeNumber)
	case typeInteger:
		return parseNumber(node, TypeInteger)
	case typeString:
		return parseString(node, metadata)
	case typeBoolean:
		return parseBoolean(node)
	case typeNull:
		return parseNull(node)
	default:
		return NullGenerator{}, nil
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
		Flags: syntax.PerlX,
	}

	if opts.RegexStringOptions == nil {
		parseOpts.RegexStringOptions = defaultRegexOpts
	}

	if opts.RegexPatternPropertyOptions == nil {
		parseOpts.RegexPatternPropertyOptions = defaultRegexOpts
	}

	return parseOpts
}
