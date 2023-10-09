package chaff_test

import (
	"testing"

	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestComplexCase(t *testing.T){
	t.Parallel()
	//test.TestJsonSchemaDir(t, "test_data/complex", 100)
	test.TestJsonSchema(t, "test_data/complex/dart-test.json", 100)
}