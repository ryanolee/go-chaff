package chaff_test

import (
	"testing"

	"github.com/ryanolee/go-chaff"
	test "github.com/ryanolee/go-chaff/internal/test_utils"
	"github.com/ryanolee/go-chaff/test_data/document/http_schema_server"
)

func getDocumentChaffConfig() *chaff.ParserOptions {
	return &chaff.ParserOptions{
		DocumentFetchOptions: chaff.DocumentFetchOptions{
			FileSystemFetchOptions: chaff.FileSystemFetchOptions{
				Enabled:      true,
				AllowedPaths: []string{"test_data/document/file"},
			},
		},
	}
}

func TestDocumentClusterSimple(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDirWithConfig(t, "test_data/document/file/cluster1_simple_refs", 100, getDocumentChaffConfig(), func() *chaff.GeneratorOptions {
		return &chaff.GeneratorOptions{
			MaximumGenerationSteps: 100,
			MaximumOneOfAttempts:   10000,
		}
	})
}

func TestDocumentClusterCyclic(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDirWithConfig(t, "test_data/document/file/cluster2_cyclic", 100, getDocumentChaffConfig(), func() *chaff.GeneratorOptions {
		return &chaff.GeneratorOptions{
			BypassCyclicReferenceCheck: true,
			MaximumReferenceDepth:      9999,
			MaximumGenerationSteps:     100,
		}
	})
}

func TestDocumentClusterPolymorphic(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDirWithConfig(t, "test_data/document/file/cluster3_polymorphic", 100, getDocumentChaffConfig(), nil)
}

func TestDeeplyNestedDocumentCluster(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDirWithConfig(t, "test_data/document/file/cluster4_deep_nested", 100, getDocumentChaffConfig(), nil)
}

func TestHttpDocuments(t *testing.T) {
	server, err := http_schema_server.StartTestServer()
	if err != nil {
		t.Fatalf("Failed to start test HTTP schema server: %s", err)
	}
	defer server.Stop()

	t.Parallel()
	test.TestJsonSchema(t, "test_data/document/http_schema_server/schemas/main.json", 100, &chaff.ParserOptions{
		DocumentFetchOptions: chaff.DocumentFetchOptions{
			HTTPFetchOptions: chaff.HTTPFetchOptions{
				Enabled: true,
				AllowedHosts: []string{
					"127.0.0.1:8080",
				},
				AllowInsecure: true,
			},
		},
		RelativeTo: server.BaseURL,
	}, func() *chaff.GeneratorOptions {
		return &chaff.GeneratorOptions{
			BypassCyclicReferenceCheck: false,
			MaximumOneOfAttempts:       100000,
			MaximumGenerationSteps:     1000,
			CutoffGenerationSteps:      200000,
		}
	})
}
