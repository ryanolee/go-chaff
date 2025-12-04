package chaff_test

import (
	"testing"

	"github.com/ryanolee/go-chaff"
	test "github.com/ryanolee/go-chaff/internal/test_utils"
)

func TestComplex(t *testing.T) {
	// @todo This is expected to fail as it is a property based test against
	//       all the schemas in schema store. It is a good indicator for current support / finding edge cases
	//       but should be excluded from typical unit tests. This can also take a very long time to run.
	t.Parallel()
	test.TestJsonSchemaDirWithConfig(t, "test_data/complex/", 10, &chaff.ParserOptions{
		DocumentFetchOptions: chaff.DocumentFetchOptions{
			HTTPFetchOptions: chaff.HTTPFetchOptions{
				Enabled: true,
				AllowedHosts: []string{
					"json.schemastore.org",
				},
			},
		},
		RelativeTo: "https://json.schemastore.org/",
	},
		func() *chaff.GeneratorOptions {
			return &chaff.GeneratorOptions{
				MaximumGenerationSteps:     100,
				BypassCyclicReferenceCheck: true,
			}
		})
}
