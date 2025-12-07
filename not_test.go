package chaff_test

import (
	"testing"

	"github.com/ryanolee/go-chaff"
	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestNot(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDirWithConfig(t, "test_data/not", 100, nil, func() *chaff.GeneratorOptions {
		return &chaff.GeneratorOptions{
			MaximumGenerationSteps: 100,
		}
	})
}
