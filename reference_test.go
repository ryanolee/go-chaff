package chaff_test

import (
	"testing"

	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestReference(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDir(t, "test_data/reference", 100)
}
