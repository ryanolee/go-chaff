package chaff

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

		// Maps resolved $id URIs to the real document + JSON pointer path
		// where the sub-schema lives, avoiding document duplication.
		idAliases map[string]idAlias
	}

	// idAlias maps a resolved $id URI back to the parent document and the
	// JSON pointer path within that document. For example, a $defs entry
	// with "$id": "color" in https://example.com/base.json produces:
	//   idAlias{documentId: "https://example.com/base.json", path: "#/$defs/color"}
	idAlias struct {
		documentId string // The real document containing this sub-schema
		path       string // JSON pointer within documentId, e.g. "#/$defs/color"
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

	resolver := &documentResolver{
		// Setup the root document
		documentCurrentlyBeingParsedId: opts.RelativeTo,
		parsedExternalDocuments: map[string]bool{
			resolvedRootDocumentId: true,
		},
		documents: map[string]*schemaNode{
			resolvedRootDocumentId: rootDocument,
		},
		documentFetchers: documentFetchers,
		idAliases:        make(map[string]idAlias),
	}

	// Collect $id aliases from the root document tree so that relative
	// $ref values (e.g. $ref: "color") can be resolved without I/O.
	baseURI := resolvedRootDocumentId
	if rootDocument.Id != nil && isAbsoluteURI(*rootDocument.Id) {
		baseURI = *rootDocument.Id
	}

	// Merge into existing idAliases
	for resolvedId, alias := range collectSubSchemaIds(baseURI, resolvedRootDocumentId, rootDocument) {
		resolver.idAliases[resolvedId] = alias
	}

	return resolver, nil
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
	if !hasDocument || documentID == "" {
		return r.GetCurrentScope(), path, nil
	}

	return r.resolveDocumentRef(documentID, path)
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
	if !hasDocument || documentID == "" {
		return r.GetCurrentScope(), path, nil
	}

	resolvedDocId, resolvedPath, err := r.resolveDocumentRef(documentID, path)
	if err != nil {
		return "", "", err
	}

	// Queue document for parsing if it hasn't already been parsed
	if !funk.ContainsString(r.externalDocumentsThatNeedParsing, resolvedDocId) && r.pathWouldRequireNewDocumentParsed(resolvedDocId) {
		r.externalDocumentsThatNeedParsing = append(r.externalDocumentsThatNeedParsing, resolvedDocId)
	}

	return resolvedDocId, resolvedPath, nil
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
	if !hasDocument || documentID == "" {
		subNode, err := resolveSubReferencePath(r.documents[r.GetCurrentScope()], path, "")
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve sub-reference path '%s' in document '%s': %w", path, r.GetCurrentScope(), err)
		}
		return subNode, fmt.Sprintf("%s%s", r.GetCurrentScope(), path), nil
	}

	resolvedDocId, resolvedPath, err := r.resolveDocumentRef(documentID, path)
	if err != nil {
		return nil, "", err
	}

	document, documentLoaded := r.documents[resolvedDocId]

	if !documentLoaded {
		document, err = r.resolveDocument(resolvedDocId)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve document '%s': %w", documentID, err)
		}
	}

	subNode, err := resolveSubReferencePath(document, resolvedPath, "")
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve sub-reference path '%s' in document '%s': %w", resolvedPath, resolvedDocId, err)
	}

	// Shift the document scope so base url resolution works correctly
	r.SetDocumentBeingResolved(resolvedDocId)

	return subNode, fmt.Sprintf("%s%s", resolvedDocId, resolvedPath), nil
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

	// Collect $id aliases from the new document tree.
	baseURI := id
	if node.Id != nil && isAbsoluteURI(*node.Id) {
		baseURI = *node.Id
	}
	for resolvedId, alias := range collectSubSchemaIds(baseURI, id, node) {
		if _, exists := r.idAliases[resolvedId]; !exists {
			r.idAliases[resolvedId] = alias
		}
	}
}

// collectSubSchemaIds marshals a schema node to a generic map and recursively
// walks the entire structure looking for "$id" entries. Each found $id is
// resolved against its nearest ancestor base URI and recorded as an idAlias.
func collectSubSchemaIds(baseURI string, documentId string, node *schemaNode) map[string]idAlias {
	data, err := json.Marshal(node)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	result := make(map[string]idAlias)
	baseAtPath := map[string]string{"": baseURI}

	WalkSchema(raw, "", func(n map[string]interface{}, path string) {
		idVal, ok := n["$id"]
		if !ok {
			return
		}
		idStr, ok := idVal.(string)
		if !ok || idStr == "" {
			return
		}
		resolved := resolveRelativeURI(nearestBaseURI(baseAtPath, path), idStr)
		if resolved == "" {
			return
		}
		baseAtPath[path] = resolved
		if path == "" {
			return // root $id is the document identity, not an alias
		}
		if _, exists := result[resolved]; exists {
			return
		}
		result[resolved] = idAlias{documentId: documentId, path: "#" + path}
	})

	return result
}

// nearestBaseURI walks up the JSON pointer path to find the closest ancestor
// that established a base URI via $id.
func nearestBaseURI(baseAtPath map[string]string, path string) string {
	for p := path; ; {
		if base, ok := baseAtPath[p]; ok {
			return base
		}
		i := strings.LastIndex(p, "/")
		if i < 0 {
			break
		}
		p = p[:i]
	}
	return baseAtPath[""]
}

// resolveRelativeURI resolves rel against base using standard URL resolution.
// Returns "" if parsing fails.
func resolveRelativeURI(base, rel string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	relURL, err := url.Parse(rel)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(relURL).String()
}

// isAbsoluteURI returns true when s looks like an absolute URI (has a scheme).
func isAbsoluteURI(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != ""
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

	// Mark as parsed before parseRoot so that intra-document $id-based
	// refs resolved during parsing don't re-queue this document.
	r.parsedExternalDocuments[nextDocumentId] = true

	_, err = parseRoot(*documentNode, metadata)

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

// resolveDocumentRef resolves a document reference to its canonical document
// ID and path. It checks the $id alias table first (a lightweight map of
// resolved $id URI → document + JSON pointer), falling back to I/O-based
// fetchers only if no alias matches.
func (r *documentResolver) resolveDocumentRef(documentID string, refPath string) (string, string, error) {
	// Check $id alias table first — resolves bare-name refs like "color"
	// to the real document + JSON pointer path with no I/O.
	resolved := resolveRelativeURI(r.GetCurrentScope(), documentID)
	if resolved != "" {
		if alias, exists := r.idAliases[resolved]; exists {
			return alias.documentId, composeJsonPointerPaths(alias.path, refPath), nil
		}
	}

	// No alias found — fall back to I/O-based fetchers.
	if r.HasNoFetchers() {
		return r.GetCurrentScope(), refPath, nil
	}

	fetcher, err := r.getFetcherForRef(documentID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get document fetcher for document '%s': %w", documentID, err)
	}

	resolvedDocumentId, err := fetcher.resolveDocumentId(r.GetCurrentScope(), documentID)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve document ID for reference '%s': %w", documentID, err)
	}

	return resolvedDocumentId, refPath, nil
}

// composeJsonPointerPaths appends the sub-path from a $ref onto an alias's
// base path. For example, alias "#/$defs/color" + ref "#/type" = "#/$defs/color/type".
// If refPath is "#" (root of the aliased resource), the alias path is returned as-is.
func composeJsonPointerPaths(aliasPath string, refPath string) string {
	if refPath == "#" || refPath == "" {
		return aliasPath
	}
	// refPath is "#/something" — strip the "#" to get "/something"
	return aliasPath + refPath[1:]
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
