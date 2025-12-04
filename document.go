package chaff

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/thoas/go-funk"
)

type (
	documentResolver struct {
		documents                        map[string]*schemaNode
		externalDocumentsThatNeedParsing []string

		documentCurrentlyBeingParsedId string
		parsedExternalDocuments        map[string]bool

		documentCurrentlyBeingResolvedId string

		// A map of protocols to their associated document fetchers
		documentFetchers map[string]*documentFetcherInterface
	}

	documentFetcherInterface interface {
		fetchDocument(ref string) (*schemaNode, error)
		resolveDocumentId(relativeTo string, ref string) (string, error)
	}

	// Used for rewriting references simply
	genericNode map[string]interface{}
)

var documentRegex = regexp.MustCompile("^(?P<document>(?:[a-zA-Z][a-zA-Z0-9+.-]*:)?[^#]*)?(?P<path>#.*)?$")

// Random UUID generated for the root document ID to prevent clashes with external document IDs
const rootDocumentId = "8dabc98a-527b-4f08-baba-315beb368097.json"

func newDocumentResolver(opts ParserOptions, rootDocument *schemaNode) (*documentResolver, error) {
	documentFetchers := make(map[string]*documentFetcherInterface)
	if opts.DocumentFetchOptions.HTTPFetchOptions.Enabled {
		httpFetcher, err := NewHttpDocumentFetcher(opts.DocumentFetchOptions.HTTPFetchOptions)
		if err != nil {
			return nil, err
		}
		documentFetchers["http"] = &httpFetcher
		documentFetchers["https"] = &httpFetcher
	}

	if opts.DocumentFetchOptions.FileSystemFetchOptions.Enabled {
		fsFetcher, err := NewFileSystemDocumentFetcher(opts.DocumentFetchOptions.FileSystemFetchOptions)
		if err != nil {
			return nil, err
		}
		documentFetchers["file"] = &fsFetcher
	}

	// Set relative to to current working directory if not set
	resolvedRootDocumentId := rootDocumentId

	if opts.RelativeTo == "" {
		cwd, err := os.Getwd()
		if err != nil {
			opts.RelativeTo = "file://./"
		} else {
			opts.RelativeTo = "file://" + cwd + "/"
		}

		resolvedRootDocumentId = filepath.Join(opts.RelativeTo, rootDocumentId)
	} else {
		resolvedRootDocumentId = opts.RelativeTo
	}

	return &documentResolver{
		// Setup the root document
		documentCurrentlyBeingParsedId: opts.RelativeTo,
		parsedExternalDocuments: map[string]bool{
			resolvedRootDocumentId: true,
		},
		documents: map[string]*schemaNode{
			resolvedRootDocumentId: rootDocument,
		},
		documentFetchers: documentFetchers,
	}, nil
}

func (r *documentResolver) GetDocumentIdCurrentlyBeingParsed() string {
	return r.documentCurrentlyBeingParsedId
}

// Compile time function to get the document ID currently being resolved
func (r *documentResolver) GetCurrentScope() string {
	if r.documentCurrentlyBeingResolvedId != "" {
		return r.documentCurrentlyBeingResolvedId
	}

	return r.documentCurrentlyBeingParsedId
}

func (r *documentResolver) HasNoFetchers() bool {
	return len(r.documentFetchers) == 0
}

func (r *documentResolver) ResolveDocumentIdAndPath(ref string) (string, string, error) {
	matches := util.RegexMatchNamedCaptureGroups(documentRegex, ref)
	documentID, hasDocument := matches["document"]
	path, hasPath := matches["path"]
	if !hasPath || path == "" {
		path = "#"
	}

	// If we have no document, it is a local or relative reference and can be handled as such
	if !hasDocument || documentID == "" || r.HasNoFetchers() {
		return r.GetCurrentScope(), path, nil
	}

	fetcher, err := r.getFetcherForRef(ref)
	if err != nil {
		return "", "", fmt.Errorf("failed to get document fetcher for document '%s': %w", ref, err)
	}

	resolvedDocumentId, err := fetcher.resolveDocumentId(r.GetCurrentScope(), documentID)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve document ID for reference '%s': %w", ref, err)
	}

	return resolvedDocumentId, path, nil
}

// Traverses a schema node and rewrites any $ref fields to be relative to the given document ID
// useful for when $ref tags need to be later merged during normal document parsing and not the "fast path" resolution
// required when merging documents
func (r *documentResolver) RewriteReferencesRelativeToDocument(node schemaNode) (schemaNode, error) {
	jsonData, err := json.Marshal(node)
	if err != nil {
		return schemaNode{}, fmt.Errorf("failed to marshal schema node to JSON for reference rewriting: %w", err)
	}

	var genericNode genericNode
	if err = json.Unmarshal(jsonData, &genericNode); err != nil {
		return schemaNode{}, fmt.Errorf("failed to unmarshal schema node to generic node for reference rewriting: %w", err)
	}

	genericNode, err = r.rewriteReferencesRelativeToDocumentInGenericNode(genericNode)
	if err != nil {
		return schemaNode{}, fmt.Errorf("failed to rewrite references relative to document '%s': %w", r.GetCurrentScope(), err)
	}

	rewrittenJsonData, err := json.Marshal(genericNode)
	if err != nil {
		return schemaNode{}, fmt.Errorf("failed to marshal rewritten generic node to JSON: %w", err)
	}

	newNode := schemaNode{}
	if err = json.Unmarshal(rewrittenJsonData, &newNode); err != nil {
		return schemaNode{}, fmt.Errorf("failed to unmarshal rewritten JSON back to schema node: %w", err)
	}

	return newNode, nil
}

func (r *documentResolver) rewriteReferencesRelativeToDocumentInGenericNode(genericNode genericNode) (genericNode, error) {
	for key, value := range genericNode {
		if key == "$ref" {
			refStr, ok := value.(string)

			// If the value under $ref is not a string, skip it
			// for polymorphic schemas that may have non-string $ref values
			// { "properties": {"$ref": {...}}
			if !ok {
				return genericNode, nil
			}

			documentId, path, err := r.ResolveDocumentIdAndPath(refStr)
			if err != nil {
				return genericNode, fmt.Errorf("failed to resolve document ID and path for ref '%s': %w", refStr, err)
			}

			// Rewrite the reference to be relative to the given document ID
			genericNode["$ref"] = fmt.Sprintf("%s%s", documentId, path)
		}

		// Recursively traverse nested objects
		switch v := value.(type) {
		case map[string]interface{}:
			rewrittenNode, err := r.rewriteReferencesRelativeToDocumentInGenericNode(v)
			if err != nil {
				return genericNode, err
			}
			genericNode[key] = rewrittenNode
		case []interface{}:
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					rewrittenItem, err := r.rewriteReferencesRelativeToDocumentInGenericNode(itemMap)
					if err != nil {
						return genericNode, err
					}
					v[i] = rewrittenItem
				}
			}
			genericNode[key] = v
		}
	}

	return genericNode, nil
}

func (r *documentResolver) SetDocumentBeingResolved(documentId string) {
	r.documentCurrentlyBeingResolvedId = documentId
}

func (r *documentResolver) HandleDeferredReferenceResolution(ref string, metadata *parserMetadata) (string, string, error) {
	matches := util.RegexMatchNamedCaptureGroups(documentRegex, ref)
	documentID, hasDocument := matches["document"]
	path, hasPath := matches["path"]
	if !hasPath || path == "" {
		path = "#"
	}

	// If we have no document, it is a local or relative reference and can be handled as such
	if !hasDocument || documentID == "" || r.HasNoFetchers() {
		return r.GetCurrentScope(), ref, nil
	}

	fetcher, err := r.getFetcherForRef(documentID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get document fetcher for document '%s': %w", ref, err)
	}

	resolvedDocumentId, err := fetcher.resolveDocumentId(r.GetCurrentScope(), documentID)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve document ID for reference '%s': %w", ref, err)
	}

	// Queue document for parsing if it hasn't already been parsed
	if !funk.ContainsString(r.externalDocumentsThatNeedParsing, resolvedDocumentId) && r.pathWouldRequireNewDocumentParsed(resolvedDocumentId) {
		r.externalDocumentsThatNeedParsing = append(r.externalDocumentsThatNeedParsing, resolvedDocumentId)
	}

	return resolvedDocumentId, path, nil
}

/**
 * Fetches if a path would require a new document resolution parsing in order for the
 * reference to be correctly resolved. Returns if the document needs parsing, and the document ID
 * itself.
 */
func (r *documentResolver) pathWouldRequireNewDocumentParsed(documentId string) bool {
	hasBeenParsed, ok := r.parsedExternalDocuments[documentId]
	return !ok || !hasBeenParsed
}

func (r *documentResolver) ResolvePath(metadata *parserMetadata, ref string) (*schemaNode, string, error) {
	matches := util.RegexMatchNamedCaptureGroups(documentRegex, ref)
	documentID, hasDocument := matches["document"]
	path, hasPath := matches["path"]

	if !hasDocument && !hasPath {
		return nil, "", errors.New("invalid $ref format, must contain document or path")
	}

	if !hasPath || path == "" {
		path = "#"
	}

	// If we have no document, it is a local or relative reference and can be handled as such
	if !hasDocument || documentID == "" || r.HasNoFetchers() {
		subNode, err := resolveSubReferencePath(r.documents[r.GetCurrentScope()], path, "")
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve sub-reference path '%s' in document '%s': %w", path, r.GetCurrentScope(), err)
		}
		return subNode, fmt.Sprintf("%s%s", r.GetCurrentScope(), path), nil
	}

	fetcher, err := r.getFetcherForRef(documentID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get document fetcher for document '%s': %w", ref, err)
	}

	resolvedDocumentId, err := fetcher.resolveDocumentId(r.GetCurrentScope(), documentID)
	if resolvedDocumentId == "" {
		return nil, "", fmt.Errorf("failed to resolve document ID for reference '%s': %w", ref, err)
	}

	document, documentLoaded := r.documents[resolvedDocumentId]

	if !documentLoaded {
		document, err = r.resolveDocument(resolvedDocumentId)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve document '%s': %w", documentID, err)
		}
	}

	subNode, err := resolveSubReferencePath(document, path, "")
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve sub-reference path '%s' in document '%s': %w", path, documentID, err)
	}

	// Shift the document scope so base url resolution works correctly
	r.SetDocumentBeingResolved(resolvedDocumentId)

	return subNode, fmt.Sprintf("%s%s", resolvedDocumentId, path), nil
}

func (r *documentResolver) addDocument(id string, node *schemaNode) {
	// Skip if document already exists
	if _, exists := r.documents[id]; exists {
		return
	}

	// External documents are marked as unparsed initially
	// until the need to be parsed in whole due to a "generation time" reference
	// points into said document
	r.documents[id] = node
	r.parsedExternalDocuments[id] = false
}

// Checks if there are more documents that need to be parsed
func (r *documentResolver) HasMoreDocumentsToParse() bool {
	return len(r.externalDocumentsThatNeedParsing) > 0
}

// Parses the next document in the queue
func (r *documentResolver) ParseNextDocument(metadata *parserMetadata) (string, error) {
	// Get the next document to parse and remove it from the queue
	nextDocumentId := r.externalDocumentsThatNeedParsing[0]
	r.externalDocumentsThatNeedParsing = r.externalDocumentsThatNeedParsing[1:]

	// Resolve the document
	r.documentCurrentlyBeingParsedId = nextDocumentId
	r.documentCurrentlyBeingResolvedId = nextDocumentId
	documentNode, err := r.resolveDocument(nextDocumentId)
	if err != nil {
		return nextDocumentId, fmt.Errorf("failed to resolve document '%s': %w", nextDocumentId, err)
	}

	_, err = parseRoot(*documentNode, metadata)
	// Mark document as parsed
	r.parsedExternalDocuments[nextDocumentId] = true

	if err != nil {
		return nextDocumentId, fmt.Errorf("failed to parse document '%s': %w", nextDocumentId, err)
	}

	return nextDocumentId, nil
}

// Resolves a document based on its reference
func (r *documentResolver) resolveDocument(ref string) (*schemaNode, error) {
	matches := util.RegexMatchNamedCaptureGroups(documentRegex, ref)
	documentID, hasDocument := matches["document"]

	if !hasDocument || documentID == "" || r.HasNoFetchers() {
		return nil, fmt.Errorf("invalid $ref format, must contain document, ref given '%s'", ref)
	}

	fetcher, err := r.getFetcherForRef(documentID)

	if err != nil {
		return nil, fmt.Errorf("failed to get document fetcher for document '%s': %w", ref, err)
	}

	document, err := fetcher.fetchDocument(documentID)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch document '%s': %w", ref, err)
	}

	r.addDocument(documentID, document)
	return document, nil
}

func (r *documentResolver) getFetcherForRef(documentId string) (documentFetcherInterface, error) {
	url, err := url.Parse(documentId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document URL '%s': %w", err, err)
	}

	if url.Scheme == "" {
		url, err = url.Parse(r.documentCurrentlyBeingParsedId)
		if err != nil {
			return nil, fmt.Errorf("failed to parse base document URL '%s': %w", r.documentCurrentlyBeingParsedId, err)
		}

		if url.Scheme == "" {
			return nil, fmt.Errorf("document ID '%s' has no scheme and cannot be resolved against it or it's document root '%s'", documentId, r.documentCurrentlyBeingParsedId)
		}

	}

	fetcher, ok := r.documentFetchers[url.Scheme]
	if !ok || fetcher == nil {
		return nil, fmt.Errorf("no document fetcher registered for protocol check to see if you allow the '%s://' scheme the currently supported schemes if enabled are ('file://', 'http://', 'https://')", url.Scheme)
	}

	return *fetcher, nil
}
