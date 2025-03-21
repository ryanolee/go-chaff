package chaff_test

import (
	"testing"

	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestParse(t *testing.T) {
	t.Parallel()
	test.TestJsonSchemaDir(t, "test_data/parse", 100)
}
