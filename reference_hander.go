package chaff

import (
	"fmt"
	"strings"

	"github.com/thoas/go-funk"
)

type (
	// Represents a single reference within the json schema
	reference struct {
		Path       string
		Document   string
		Generator  Generator
		SchemaNode schemaNode
	}

	// Used to handle references in the parsed structure of the json structure
	// This gets populated as nodes are parsed
	referenceHandler struct {
		documentResolver *documentResolver
		CurrentPath      string
		References       map[string]map[string]reference
		Errors           map[string]map[string]error
	}

	// This struct used to track a stack of resolved references
	// It is useful for handling circular references / cases where the generator could otherwise run forever
	referenceResolver struct {
		resolutions []string
	}
)

func newReferenceHandler(documentResolver *documentResolver) *referenceHandler {
	return &referenceHandler{
		CurrentPath:      "#",
		References:       make(map[string]map[string]reference),
		Errors:           make(map[string]map[string]error),
		documentResolver: documentResolver,
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
	documentId := h.documentResolver.GetDocumentIdCurrentlyBeingParsed()
	if _, exists := h.References[documentId]; !exists {
		h.References[documentId] = make(map[string]reference)
	}

	h.References[documentId][path] = reference{
		Path:       path,
		SchemaNode: node,
		Document:   documentId,
		Generator:  generator,
	}
}

func (h *referenceHandler) HandleError(err error, metadata *parserMetadata) {
	documentId := h.documentResolver.GetDocumentIdCurrentlyBeingParsed()
	h.Errors[h.CurrentPath][documentId] = err
}

func (h *referenceHandler) Lookup(documentId string, path string) (reference, bool) {
	ref, ok := h.References[documentId][path]
	return ref, ok
}

func (r *referenceResolver) PushRefResolution(document string, reference string) {
	r.resolutions = append(r.resolutions, fmt.Sprintf("%s|%s", document, reference))
}

func (r *referenceResolver) PopRefResolution() {
	r.resolutions = r.resolutions[:len(r.resolutions)-1]
}

// Returns the current resolution in the form "document", "referencePath"
func (r *referenceResolver) GetCurrentResolution() (string, string) {
	if len(r.resolutions) == 0 {
		return "", ""
	}
	res := r.resolutions[len(r.resolutions)-1]
	parts := strings.SplitN(res, "|", 2)
	return parts[0], parts[1]
}

func (r *referenceResolver) HasResolved(document string, reference string) bool {
	return funk.ContainsString(r.resolutions, fmt.Sprintf("%s|%s", document, reference))
}

func (r *referenceResolver) GetResolutions() []string {
	return r.resolutions
}

func (r *referenceResolver) GetFormattedResolutions() string {
	return strings.Join(r.resolutions, " -> \n")
}
