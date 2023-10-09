package chaff

import (
	"strings"

	"github.com/thoas/go-funk"
)

type (
	// Represents a single reference within the json schema
	reference struct {
		Path       string
		Generator  Generator
		SchemaNode schemaNode
	}

	// Used to handle references in the parsed structure of the json structure
	// This gets populated as nodes are parsed
	referenceHandler struct {
		CurrentPath string
		References  map[string]reference
		Errors      map[string]error
	}

	// This struct used to track a stack of resolved references
	// It is useful for handling circular references / cases where the generator could otherwise run forever
	referenceResolver struct {
		resolutions []string
	}
)

func newReferenceHandler() referenceHandler {
	return referenceHandler{
		CurrentPath: "#",
		References:  make(map[string]reference),
		Errors:      make(map[string]error),
	}
}

func (h *referenceHandler) ParseNodeInScope(scope string, node schemaNode, metadata *parserMetadata) (Generator, error) {
	h.PushToPath(scope)
	generator, err := parseNode(node, metadata)
	h.PopFromPath(scope)
	return generator, err
}

func (h *referenceHandler) PushToPath(pathPart string) {
	h.CurrentPath += pathPart
}

func (h *referenceHandler) PopFromPath(pathPart string) {
	h.CurrentPath = h.CurrentPath[:len(h.CurrentPath)-len(pathPart)]
}

func (h *referenceHandler) AddReference(node schemaNode, generator Generator) {
	h.AddIdReference(h.CurrentPath, node, generator)
}

func (h *referenceHandler) AddIdReference(path string, node schemaNode, generator Generator) {
	h.References[path] = reference{
		Path:       path,
		SchemaNode: node,
		Generator:  generator,
	}
}

func (h *referenceHandler) HandleError(err error) {
	h.Errors[h.CurrentPath] = err
}

func (h *referenceHandler) Lookup(path string) (reference, bool) {
	Reference, ok := h.References[path]
	return Reference, ok
}

func (r *referenceResolver) PushRefResolution(reference string) {
	r.resolutions = append(r.resolutions, reference)
}

func (r *referenceResolver) PopRefResolution() {
	r.resolutions = r.resolutions[:len(r.resolutions)-1]

}

func (r *referenceResolver) HasResolved(reference string) bool {
	return funk.ContainsString(r.resolutions, reference)
}

func (r *referenceResolver) GetResolutions() []string {
	return r.resolutions
}

func (r *referenceResolver) GetFormattedResolutions() string {
	return strings.Join(r.resolutions, " -> \n")
}
