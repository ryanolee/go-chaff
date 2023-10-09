package chaff

import (
	"strings"

	"github.com/ryanolee/go-chaff/internal/regen"
)

const fullStringLiteral = "[~{FULL_STOP_LITERAL}~]"

type (
	RegexOptions struct {
		regen.GeneratorArgs
	}
)

func newRegexGenerator(pattern string, opts *regen.GeneratorArgs) (regen.Generator, error) {
	if opts.SuppressRandomBytes {
		pattern = strings.ReplaceAll(pattern, "\\.", fullStringLiteral)
		pattern = strings.ReplaceAll(pattern, ".", "\\w")
		pattern = strings.ReplaceAll(pattern, fullStringLiteral, "\\.")
	}

	return regen.NewGenerator(pattern, opts)
}
