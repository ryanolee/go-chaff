package chaff_test

import (
	"testing"

	"github.com/ryanolee/go-chaff"
	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestReference(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDir(t, "test_data/reference", 100)
}

func TestReferenceSelfReferencing(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDirWithConfig(t, "test_data/reference/self_referencing", 100, nil, func() *chaff.GeneratorOptions {
		return &chaff.GeneratorOptions{
			BypassCyclicReferenceCheck: true,
			MaximumReferenceDepth:      10,
		}
	})
}
