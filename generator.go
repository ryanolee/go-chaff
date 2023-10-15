package chaff

import (
	"fmt"

	"github.com/ryanolee/go-chaff/rand"
)

type (
	Generator interface {
		fmt.Stringer
		Generate(*GeneratorOptions) interface{}
	}

	GeneratorOptions struct {
		// The source of randomness to use for the given generation.
		// Please note that some parts of the generators use different sources of randomness.
		// ("regex" generation and "format" strings)
		Rand *rand.RandUtil

		// The default minimum number value
		DefaultNumberMinimum int

		// The default maximum number value
		DefaultNumberMaximum int

		// The default minimum String length
		DefaultStringMinLength int

		// The default maximum String length
		DefaultStringMaxLength int

		// The default minimum array length
		DefaultArrayMinItems int

		// The default maximum array length
		// This will be set min + this inf the event a minimum value is set
		DefaultArrayMaxItems int

		// The default minimum object properties (Will be ignored if there are fewer properties available)
		DefaultObjectMinProperties int

		// The default maximum object properties (Will be ignored if there are fewer properties available)
		DefaultObjectMaxProperties int

		// The maximum number of references to resolve at once (Default: 10)
		MaximumReferenceDepth int

		// In the event that schemas are recursive there is a good chance the generator
		// can run forever. This option will bypass the check for cyclic references
		// Please defer to the MaximumReferenceDepth option if possible when using this
		BypassCyclicReferenceCheck bool

		// Used to keep track of references during a resolution cycle (Used internally and can be ignored)
		ReferenceResolver referenceResolver

		// Though technically in some cases a schema may allow for additional
		// values it might not always be desireable. this option suppresses fallback_n values
		// so that they will only appear to make up a "minimum value" forces them to
		SuppressFallbackValues bool
	}
)

func withGeneratorOptionsDefaults(options GeneratorOptions) *GeneratorOptions {
	return &GeneratorOptions{
		// General
		Rand: options.Rand,

		// Number
		DefaultNumberMinimum: getInt(options.DefaultNumberMinimum, 0),
		DefaultNumberMaximum: getInt(options.DefaultNumberMaximum, 100),

		// String
		DefaultStringMinLength: getInt(options.DefaultStringMinLength, 0),
		DefaultStringMaxLength: getInt(options.DefaultStringMaxLength, 100),

		// Array
		DefaultArrayMinItems: getInt(options.DefaultArrayMinItems, 0),
		DefaultArrayMaxItems: getInt(options.DefaultArrayMaxItems, 10),

		// Object
		DefaultObjectMinProperties: getInt(options.DefaultObjectMinProperties, 0),
		DefaultObjectMaxProperties: getInt(options.DefaultObjectMaxProperties, 10),
		SuppressFallbackValues:     getBool(options.SuppressFallbackValues, true),

		// References
		BypassCyclicReferenceCheck: getBool(options.BypassCyclicReferenceCheck, false),
		MaximumReferenceDepth:      getInt(options.MaximumReferenceDepth, 10),
		ReferenceResolver:          referenceResolver{},
	}
}
