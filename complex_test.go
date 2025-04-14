package chaff_test

import (
	"testing"

	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestComplex(t *testing.T) {
	// @todo This is expected to fail as it is a property based test against
	//       all the schemas in schema store. It is a good indicator for current support / finding edge cases
	//       but should be excluded from typical unit tests. This can also take a very long time to run.
	t.Parallel()
	test.TestJsonSchemaDir(t, "test_data/complex", 10)
}
