package chaff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Based on a reference path and a schema node, resolve the schema node at the reference path
func resolveReferencePath(node schemaNode, refPath string) (schemaNode, error) {
	
	if refPath == "" {
		return node, nil
	}

	return resolveSubReferencePath(node, refPath, "")
}

// Recursively resolve a reference path based on a schema node (Internally used by resolveReferencePath)
func resolveSubReferencePath(node schemaNode, refPath string, resolvedPath string) (schemaNode, error) {
	pathPart, path := getReferencePathToken(refPath)
	resolvedPath = resolvedPath + "/" + pathPart
	
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
			return schemaNode{}, fmt.Errorf("[%s] No schema node for additional properties", resolvedPath)
		}

		return resolveSubReferencePath(*node.AdditionalProperties.Schema, path, resolvedPath)
	
	// Array
	case "items":		
		part, _ := getReferencePathToken(path)
		match, err := regexp.MatchString(`^\d+$`, part)
		if !match || err != nil {
			return resolveSubReferencePath(*node.Items.Node, path, resolvedPath)
		}

		return resolveReferenceSlice(node.Items.Nodes, path, resolvedPath)
	case "prefixItems":
		return resolveReferenceSlice(node.PrefixItems, path, resolvedPath)
	case "additionalItems":
		return resolveSubReferencePath(*node.AdditionalItems, path, resolvedPath)
	

	// Combinations
	case "allOf":
		return resolveReferenceSlice(node.AllOf, path, resolvedPath)
	case "anyOf":
		return resolveReferenceSlice(node.AnyOf, path, resolvedPath)
	case "oneOf":
		return resolveReferenceSlice(node.OneOf, path, resolvedPath)

		// Definitions
	case "definitions":
		return resolveReferenceProperty(node.Definitions, path, resolvedPath)
	case "$defs":
		return resolveReferenceProperty(node.Defs, path, resolvedPath)

	// Root
	case "#":
		return resolveSubReferencePath(node, path, resolvedPath)
	default:
		return schemaNode{}, fmt.Errorf("[%s] Invalid reference path", resolvedPath)
	}
}

// Resolve a reference property based on a map of schema nodes
func resolveReferenceProperty(nodes map[string]schemaNode, path string, resolvedPath string) (schemaNode, error) {
	propertyName, path := getReferencePathToken(path)
	resolvedPath = resolvedPath + "/" + propertyName
	node, ok := nodes[propertyName]
	if !ok {
		return schemaNode{}, fmt.Errorf("[%s] Property %s not found", resolvedPath,  propertyName)
	}

	return resolveSubReferencePath(node, path, resolvedPath)
}

// Resolve a reference based on a slice of schema nodes and a path
func resolveReferenceSlice(nodes []schemaNode, path string, resolvedPath string)(schemaNode, error){
	part, itemPath := getReferencePathToken(path)
	resolvedPath = resolvedPath + "/" + part
	partInt, err := strconv.Atoi(part)
	if err != nil {
		return schemaNode{}, fmt.Errorf("[%s] Invalid array index (Must be a number) %s", resolvedPath, part)
	}

	if len(nodes) > partInt || partInt < 0 {
		return schemaNode{}, fmt.Errorf("[%s] Array index out of bounds %d", resolvedPath, partInt)
	}

	node := nodes[partInt]
	
	return resolveSubReferencePath(node, itemPath, resolvedPath)
}

var pathDeliminator = regexp.MustCompile(`\/`)  

// Get the first token of a reference path and the rest of the path
func getReferencePathToken(pathRef string) (string, string){
	if !strings.Contains(pathRef, "/"){
		return pathRef, ""
	}
	
	refPathParts := pathDeliminator.Split(pathRef, 2)
	return refPathParts[0], refPathParts[1]
}