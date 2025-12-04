package chaff_test

import (
	"testing"

	"github.com/ryanolee/go-chaff"
	test "github.com/ryanolee/go-chaff/internal/test_utils"
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
	test.TestJsonSchemaDirWithConfig(t, "test_data/document/file/cluster1_simple_refs", 100, getDocumentChaffConfig(), nil)
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
