package chaff

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/thoas/go-funk"
)

type (
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

func NewHttpDocumentFetcher(parserConfig HTTPFetchOptions) (documentFetcherInterface, error) {
	allowedHosts := []string{}

	if !parserConfig.Enabled {
		return nil, nil
	}

	for _, host := range parserConfig.AllowedHosts {
		parsedUrl, err := url.Parse(host)

		if parsedUrl != nil && err != nil {
			allowedHosts = append(allowedHosts, parsedUrl.Host)
		} else {
			allowedHosts = append(allowedHosts, host)
		}

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
		if !funk.Contains(f.allowedHosts, resolvedUrl.Host) {
			return "", fmt.Errorf("host '%s' not allowed to be fetched from allowed hosts %s", resolvedUrl.Host, strings.Join(f.allowedHosts, ", "))
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
	if !config.Enabled {
		return nil, nil
	}
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
		return "", fmt.Errorf("failed to resolve file path for URL '%s' relative to '%s': %w", ref, relativeTo, err)
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
		panic("huh")
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
