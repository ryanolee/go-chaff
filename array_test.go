package chaff_test

import (
	"testing"

	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestArray(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDir(t, "test_data/array", 100)
}
