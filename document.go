package chaff

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	}

	documentFetcherInterface interface {
		fetchDocument(ref string) (*schemaNode, error)
		resolveDocumentId(relativeTo string, ref string) (string, error)
	}

	httpDocumentFetcher struct {
		// Allowed hosts to fetch from (If empty, all hosts are allowed)
		allowedHosts []string

		// Allow insecure connections (http)
		allowInsecure bool
	}

	fileSystemDocumentFetcher struct {
		// Overrides allowOutsideCwd to specifically allow for access to a list of paths schemas might reference
		allowedPaths []string

		// Failsafe to prevent directory traversal attacks
		allowOutsideCwd bool
	}
)

var documentRegex = regexp.MustCompile("^(?P<document>(?:[a-zA-Z][a-zA-Z0-9+.-]*:)?[^#]*)?(?P<path>#.*)?$")

// Random UUID generated for the root document ID to prevent clashes with external document IDs
const rootDocumentId = "file://8dabc98a-527b-4f08-baba-315beb368097.json"

func newDocumentResolver(opts ParserOptions, rootDocument *schemaNode) *documentResolver {
	documentFetchers := make(map[string]*documentFetcherInterface)
	if opts.DocumentFetchOptions.HTTPFetchOptions.Enabled {
		httpFetcher, _ := NewHttpDocumentFetcher(opts.DocumentFetchOptions.HTTPFetchOptions)
		documentFetchers["http"] = &httpFetcher
		documentFetchers["https"] = &httpFetcher
	}

	if opts.DocumentFetchOptions.FileSystemFetchOptions.Enabled {
		fsFetcher, _ := NewFileSystemDocumentFetcher(opts.DocumentFetchOptions.FileSystemFetchOptions)
		documentFetchers["file"] = &fsFetcher
	}

	// Set relative to to current working directory if not set
	if opts.RelativeTo == "" {
		cwd, err := os.Getwd()
		if err != nil {
			opts.RelativeTo = "file://./"
		} else {
			opts.RelativeTo = "file://" + cwd + "/"
		}
	}
	return &documentResolver{
		// Setup the root document
		documentCurrentlyBeingParsedId: opts.RelativeTo,
		parsedExternalDocuments: map[string]bool{
			rootDocumentId: true,
		},
		documents: map[string]*schemaNode{
			rootDocumentId: rootDocument,
		},
		documentFetchers: documentFetchers,
	}
}

func (r *documentResolver) GetDocumentIdCurrentlyBeingParsed() string {
	return r.documentCurrentlyBeingParsedId
}

// Compile time function to get the document ID currently being resolved
func (r *documentResolver) GetDocumentForResolvedPath(ref string) string {
	matches := util.RegexMatchNamedCaptureGroups(documentRegex, ref)
	documentID, hasDocument := matches["document"]

	if hasDocument {
		return documentID
	}

	if r.documentCurrentlyBeingResolvedId != "" {
		return r.documentCurrentlyBeingResolvedId
	}

	return r.documentCurrentlyBeingParsedId
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
		return r.documentCurrentlyBeingParsedId, ref, nil
	}

	fmt.Printf("Handling deferred reference resolution for document: %s relative to %s\n", documentID, r.documentCurrentlyBeingParsedId)

	fetcher, err := r.getFetcherForRef(documentID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get document fetcher for document '%s': %w", ref, err)
	}

	resolvedDocumentId, err := fetcher.resolveDocumentId(r.documentCurrentlyBeingParsedId, documentID)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve document ID for reference '%s': %w", ref, err)
	}
	fmt.Printf("Resolved document ID: %s \n", resolvedDocumentId)

	// Queue document for parsing if it hasn't already been parsed
	if !funk.ContainsString(r.externalDocumentsThatNeedParsing, resolvedDocumentId) && r.pathWouldRequireNewDocumentParsed(resolvedDocumentId) {
		fmt.Println("Queuing document for parsing:", resolvedDocumentId)
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

func (r *documentResolver) ResolvePath(metadata *parserMetadata, ref string) (*schemaNode, error) {
	matches := util.RegexMatchNamedCaptureGroups(documentRegex, ref)

	documentID, hasDocument := matches["document"]
	path, hasPath := matches["path"]

	if !hasDocument && !hasPath {
		return nil, errors.New("invalid $ref format, must contain document or path")
	}

	if !hasDocument {
		_, err := r.resolveDocument(documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve document '%s': %w", documentID, err)
		}
	}

	return resolveSubReferencePath(r.documents[documentID], path, "")
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

	if !hasDocument {
		return nil, errors.New("invalid $ref format, must contain document")
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

func NewHttpDocumentFetcher(parserConfig HTTPFetchOptions) (documentFetcherInterface, error) {
	allowedHosts := []string{}
	for _, host := range parserConfig.AllowedHosts {
		parsedUrl, err := url.ParseRequestURI(host)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed host '%s': %w", host, err)
		}

		if parsedUrl.Hostname() == "" {
			return nil, fmt.Errorf("invalid allowed host '%s': missing hostname", host)
		}

		allowedHosts = append(allowedHosts, parsedUrl.Hostname())
	}

	return &httpDocumentFetcher{
		allowedHosts:  allowedHosts,
		allowInsecure: parserConfig.AllowInsecure,
	}, nil
}

func (f *httpDocumentFetcher) resolveDocumentId(relativeTo string, ref string) (string, error) {
	parsedUrl, err := url.Parse(ref)

	if err != nil {
		return "", fmt.Errorf("invalid URL '%s': %w", ref, err)
	}

	baseUrl, err := url.Parse(relativeTo)
	if err != nil {
		return "", fmt.Errorf("invalid base URL '%s': %w", relativeTo, err)
	}

	// If our base url is http or https, we resolve relative references against it
	// otherwise we leave it as is
	resolvedUrl := parsedUrl
	if baseUrl.Scheme == "http" || baseUrl.Scheme == "https" {
		resolvedUrl = baseUrl.ResolveReference(parsedUrl)
	}

	if !f.allowInsecure && resolvedUrl.Scheme != "https" {
		return "", fmt.Errorf("insecure URL scheme '%s' not allowed for URL '%s'", parsedUrl.Scheme, ref)
	}

	if len(f.allowedHosts) > 0 {
		if !funk.Contains(f.allowedHosts, resolvedUrl.Hostname()) {
			return "", fmt.Errorf("host '%s' not allowed to be fetched from allowed hosts %s", parsedUrl.Hostname(), strings.Join(f.allowedHosts, ", "))
		}
	}

	return resolvedUrl.String(), nil
}

func (f *httpDocumentFetcher) fetchDocument(resolvedPath string) (*schemaNode, error) {
	resp, err := http.Get(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL '%s': %w", resolvedPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch URL '%s': received status code %d", resolvedPath, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from URL '%s': %w", resolvedPath, err)
	}

	schemaNode := &schemaNode{}
	if err = json.Unmarshal(data, schemaNode); err != nil {
		return nil, fmt.Errorf("failed to parse Schema json from URL '%s': %w", resolvedPath, err)
	}

	return schemaNode, nil

}

func NewFileSystemDocumentFetcher(config FileSystemFetchOptions) (documentFetcherInterface, error) {
	allowedRealPaths := []string{}
	for _, path := range config.AllowedPaths {
		realPath, err := getRealPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve allowed path '%s': %w", path, err)
		}

		allowedRealPaths = append(allowedRealPaths, realPath)
	}

	return &fileSystemDocumentFetcher{
		allowedPaths:    allowedRealPaths,
		allowOutsideCwd: config.AllowOutsideCwd,
	}, nil
}

func (f *fileSystemDocumentFetcher) resolveDocumentId(relativeTo string, ref string) (string, error) {
	// Read based on file:// scheme
	parsedUrl, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("invalid file URL '%s': %w", ref, err)
	}

	if parsedUrl.Scheme != "file" && parsedUrl.Scheme != "" {
		return "", fmt.Errorf("invalid file URL scheme '%s' for URL '%s'", parsedUrl.Scheme, ref)
	}

	filePath := strings.TrimPrefix(ref, "file://")

	// Resolve against the file path we are relative to
	relativeToDir := filepath.Dir(strings.TrimPrefix(relativeTo, "file://"))
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(relativeToDir, filePath)
	}

	resolvedPath, err := getRealPath(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path for URL '%s': %w", ref, err)
	}

	if !f.allowOutsideCwd {
		outsideCwd, err := f.isOutsideOfCwd(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("failed to check if path '%s' is outside of current working directory: %w", resolvedPath, err)
		}

		cwd, _ := os.Getwd()

		if outsideCwd {
			return "", fmt.Errorf("access to path '%s' outside of current working directory '%s' is not allowed", resolvedPath, cwd)
		}
	}

	outsideAllowedPaths, err := isOutsideOfAllowedPaths(resolvedPath, f.allowedPaths)
	if err != nil {
		return "", fmt.Errorf("failed to check if path '%s' is outside of allowed paths: %w", resolvedPath, err)
	}

	if outsideAllowedPaths {
		return "", fmt.Errorf("access to path '%s' is not allowed. Only paths files in the following paths are allowed: %s", resolvedPath, strings.Join(f.allowedPaths, ", "))
	}

	return "file://" + resolvedPath, nil
}

func (f *fileSystemDocumentFetcher) fetchDocument(resolvedPath string) (*schemaNode, error) {
	resolvedPath = strings.TrimPrefix(resolvedPath, "file://")
	fileData, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at path '%s': %w", resolvedPath, err)
	}

	schemaNode := &schemaNode{}
	if err = json.Unmarshal(fileData, schemaNode); err != nil {
		return nil, fmt.Errorf("failed to parse Schema json from file at path '%s': %w", resolvedPath, err)
	}

	return schemaNode, nil
}

func (f *fileSystemDocumentFetcher) isOutsideOfCwd(path string) (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get current working directory: %w", err)
	}

	diff, err := filepath.Rel(cwd, path)
	return strings.Contains(diff, ".."), err
}

func isOutsideOfAllowedPaths(path string, allowedPaths []string) (bool, error) {
	for _, allowedPath := range allowedPaths {
		matched, err := filepath.Match(filepath.Join(allowedPath, "*"), path)
		if err != nil {
			return false, fmt.Errorf("failed to match path '%s' against allowed path '%s': %w", path, allowedPath, err)
		}

		if !matched {
			return false, nil
		}
	}

	return true, nil
}

func getRealPath(path string) (string, error) {
	realPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path '%s': %w", path, err)
	}

	resolvedPath, err := filepath.EvalSymlinks(realPath)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate symlinks for path '%s': %w", realPath, err)
	}

	return resolvedPath, nil
}
