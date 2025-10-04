package chaff

import (
	"fmt"
	"regexp"

	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/santhosh-tekuri/jsonschema/v6"
	jsonschemaV6 "github.com/santhosh-tekuri/jsonschema/v6"
)

type (
	schemaManager struct {
		rootSchemaCompiler *jsonschemaV6.Compiler
	}

	internalOnlyLoader struct {
	}
)

const pathPrefix = "file://self.json"

var pathSanitiseRegex = regexp.MustCompile(`[\/\#\-\.\,\s\.]`)

// Create a new schema manager used to manage sub schema validators required for
// conditional validators where generated values must be validated against the original schema
// to ensure they conform to the original schema constraints
func newSchemaManager(schemaJson []byte) (*schemaManager, error) {
	jsonSchemaCompiler := jsonschema.NewCompiler()

	// To prevent external references from being inadvertently loaded or files from the local filesystem,
	jsonSchemaCompiler.UseLoader(internalOnlyLoader{})

	if err := jsonSchemaCompiler.AddResource(pathPrefix, util.UnmarshalJsonStringToMap(string(schemaJson))); err != nil {
		return nil, err
	}

	return &schemaManager{
		rootSchemaCompiler: jsonSchemaCompiler,
	}, nil
}

// Parses a schema node and creates a subdocument in the root schema compiler
// allowing for reference resolutions back into the original schema from any given sub-schema fragment
func (sm *schemaManager) ParseSchemaNode(parserMetadata *parserMetadata, node schemaNode, field string) (*jsonschemaV6.Schema, error) {
	currentPath := sm.normalisePath(fmt.Sprintf("%s/%s", parserMetadata.ReferenceHandler.CurrentPath, field))
	deepClonedNode := util.UnmarshalJsonStringToMap(util.MarshalJsonToString(node))
	referenceUpdatedNode := sm.replaceRefs(deepClonedNode)

	sm.rootSchemaCompiler.AddResource(pathPrefix+currentPath, referenceUpdatedNode)
	return sm.CompilePath(currentPath)
}

// Replaces all slashes with underscores and removes any hashtags or punctuation that may interfere with
// the schema compiler's ability to parse the path given it is a "file name" technically
func (sm *schemaManager) normalisePath(path string) string {
	return pathSanitiseRegex.ReplaceAllString(path, "_")
}

// Recursively replaces all $ref values in the schema with links back to the root schema
func (sm *schemaManager) replaceRefs(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		if ref, ok := v["$ref"]; ok {
			if refStr, ok := ref.(string); ok {
				v["$ref"] = pathPrefix + refStr
			}
		}
		for key, value := range v {
			v[key] = sm.replaceRefs(value)
		}
		return v
	case []interface{}:
		for i, item := range v {
			v[i] = sm.replaceRefs(item)
		}
		return v
	default:
		return v
	}
}

func (sm *schemaManager) CompilePath(path string) (*jsonschemaV6.Schema, error) {
	return sm.rootSchemaCompiler.Compile(pathPrefix + path)
}

func (l internalOnlyLoader) Load(url string) (any, error) {
	return nil, nil
}
