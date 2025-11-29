package chaff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Based on a reference path and a schema node, resolve the schema node at the reference path
func resolveReferencePath(node *schemaNode, refPath string) (*schemaNode, error) {
	if refPath == "" {
		return node, nil
	}

	return resolveSubReferencePath(node, refPath, "")
}

// Recursively resolve a reference path based on a schema node (Internally used by resolveReferencePath)
func resolveSubReferencePath(node *schemaNode, refPath string, resolvedPath string) (*schemaNode, error) {
	if node == nil {
		return nil, fmt.Errorf("[%s] No schema node found", resolvedPath)
	}

	pathPart, path := getReferencePathToken(refPath)
	resolvedPath = resolvedPath + "/" + pathPart

	if node.AnyOf != nil {
		return nil, fmt.Errorf("[%s] anyOf nodes cannot be referenced", resolvedPath)
	}

	if path == "" {
		return node, nil
	}

	switch pathPart {
	// Object
	case "properties":
		return resolveReferenceProperty(node.Properties, path, resolvedPath)
	case "patternProperties":
		return resolveReferenceProperty(node.PatternProperties, path, resolvedPath)
	case "additionalProperties":
		if node.AdditionalProperties.DisallowAdditional {
			return nil, fmt.Errorf("[%s] No schema node for additional properties", resolvedPath)
		}

		return resolveSubReferencePath(node.AdditionalProperties.Schema, path, resolvedPath)

	// Array
	case "items":
		part, _ := getReferencePathToken(path)
		match, err := regexp.MatchString(`^\d+$`, part)
		if !match || err != nil {
			return resolveSubReferencePath(node.Items.Node, path, resolvedPath)
		}

		return resolveReferenceSlice(node.Items.Nodes, path, resolvedPath)
	case "prefixItems":
		return resolveReferenceSlice(node.PrefixItems, path, resolvedPath)
	case "contains":
		return resolveSubReferencePath(node.Contains, path, resolvedPath)
	case "additionalItems":
		return resolveFalseOrSchema(node.AdditionalItems, path, resolvedPath)
	case "unevaluatedItems":
		return resolveFalseOrSchema(node.UnevaluatedItems, path, resolvedPath)

	// Combinations
	case "allOf":
		return resolveReferenceSlice(node.AllOf, path, resolvedPath)
	case "anyOf":
		return resolveReferenceSlice(node.AnyOf, path, resolvedPath)
	case "oneOf":
		return resolveReferenceSlice(node.OneOf, path, resolvedPath)
	case "not":
		return resolveSubReferencePath(node.Not, path, resolvedPath)

	// Conditionals
	case "if":
		return resolveSubReferencePath(node.If, path, resolvedPath)
	case "then":
		return resolveSubReferencePath(node.Then, path, resolvedPath)
	case "else":
		return resolveSubReferencePath(node.Else, path, resolvedPath)

		// Definitions
	case "definitions":
		return resolveReferenceProperty(node.Definitions, path, resolvedPath)
	case "$defs":
		return resolveReferenceProperty(node.Defs, path, resolvedPath)

	// Root
	case "#":
		return resolveSubReferencePath(node, path, resolvedPath)
	default:
		return nil, fmt.Errorf("[%s] Invalid reference path", resolvedPath)
	}
}

func resolveFalseOrSchema(node *schemaNodeOrFalse, path string, resolvedPath string) (*schemaNode, error) {
	if node.IsFalse {
		return nil, fmt.Errorf("[%s] No schema node for %s node is false", resolvedPath, path)
	}

	return resolveSubReferencePath(node.Schema, path, resolvedPath)
}

// Resolve a reference property based on a map of schema nodes
func resolveReferenceProperty(nodes *map[string]schemaNode, path string, resolvedPath string) (*schemaNode, error) {
	if nodes == nil {
		return nil, fmt.Errorf("[%s] No properties defined", resolvedPath)
	}

	propertyName, path := getReferencePathToken(path)
	resolvedPath = resolvedPath + "/" + propertyName
	node, ok := (*nodes)[propertyName]
	if !ok {
		return nil, fmt.Errorf("[%s] Property %s not found", resolvedPath, propertyName)
	}

	return resolveSubReferencePath(&node, path, resolvedPath)
}

// Resolve a reference based on a slice of schema nodes and a path
func resolveReferenceSlice(nodes *[]schemaNode, path string, resolvedPath string) (*schemaNode, error) {
	if nodes == nil {
		return nil, fmt.Errorf("[%s] No array items defined", resolvedPath)
	}

	part, itemPath := getReferencePathToken(path)
	resolvedPath = resolvedPath + "/" + part
	partInt, err := strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("[%s] Invalid array index (Must be a number) %s", resolvedPath, part)
	}

	if len(*nodes) > partInt || partInt < 0 {
		return nil, fmt.Errorf("[%s] Array index out of bounds %d", resolvedPath, partInt)
	}

	node := &(*nodes)[partInt]

	return resolveSubReferencePath(node, itemPath, resolvedPath)
}

var pathDeliminator = regexp.MustCompile(`\/`)

// Get the first token of a reference path and the rest of the path
func getReferencePathToken(pathRef string) (string, string) {
	if !strings.Contains(pathRef, "/") {
		return pathRef, ""
	}

	refPathParts := pathDeliminator.Split(pathRef, 2)
	return refPathParts[0], refPathParts[1]
}
