package chaff_test

import (
	"testing"

	"github.com/ryanolee/go-chaff"
	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestOneOf(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDirWithConfig(t, "test_data/oneOf", 100, nil, func() *chaff.GeneratorOptions {
		return &chaff.GeneratorOptions{
			MaximumOneOfAttempts: 10000,
		}
	})
}
