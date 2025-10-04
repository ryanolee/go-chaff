package chaff

import (
	"fmt"

	"github.com/ryanolee/go-chaff/internal/util"
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

		// The maximum number of times to attempt to generate a unique value
		// when using unique* constraints in schemas
		MaximumUniqueGeneratorAttempts int

		// The maximum number of times to attempt to satisfy "if" statements
		// before giving up
		MaximumIfAttempts int

		// The maximum number of times to attempt to satisfy "oneOf" statements
		// before giving up
		MaximumOneOfAttempts int

		// The maximum number of steps to take when generating a value
		// after which the the generator will begin to do the "bare minimum" to generate a value
		MaximumGenerationSteps int

		// The maximum number of steps to take before giving up entirely and aborting generation
		// This is a hard cap on generation steps to prevent extremely long generation times
		CutoffGenerationSteps int

		overallComplexity int
	}
)

func withGeneratorOptionsDefaults(options GeneratorOptions) *GeneratorOptions {
	randUtil := options.Rand
	if options.Rand == nil {
		randUtil = rand.NewRandUtilFromTime()
	}
	return &GeneratorOptions{
		// General
		Rand: randUtil,

		// Number
		DefaultNumberMinimum: util.GetInt(options.DefaultNumberMinimum, 0),
		DefaultNumberMaximum: util.GetInt(options.DefaultNumberMaximum, 100),

		// String
		DefaultStringMinLength: util.GetInt(options.DefaultStringMinLength, 0),
		DefaultStringMaxLength: util.GetInt(options.DefaultStringMaxLength, 100),

		// Array
		DefaultArrayMinItems: util.GetInt(options.DefaultArrayMinItems, 0),
		DefaultArrayMaxItems: util.GetInt(options.DefaultArrayMaxItems, 10),

		// Object
		DefaultObjectMinProperties: util.GetInt(options.DefaultObjectMinProperties, 0),
		DefaultObjectMaxProperties: util.GetInt(options.DefaultObjectMaxProperties, 10),
		SuppressFallbackValues:     util.GetBool(options.SuppressFallbackValues, true),

		// References
		BypassCyclicReferenceCheck: util.GetBool(options.BypassCyclicReferenceCheck, false),
		MaximumReferenceDepth:      util.GetInt(options.MaximumReferenceDepth, 10),
		ReferenceResolver:          referenceResolver{},

		// Reattempts
		MaximumUniqueGeneratorAttempts: util.GetInt(options.MaximumUniqueGeneratorAttempts, 100),
		MaximumIfAttempts:              util.GetInt(options.MaximumIfAttempts, 100),
		MaximumOneOfAttempts:           util.GetInt(options.MaximumOneOfAttempts, 100),

		// Generation
		MaximumGenerationSteps: util.GetInt(options.MaximumGenerationSteps, 100),
		CutoffGenerationSteps:  util.GetInt(options.CutoffGenerationSteps, 2000),
		overallComplexity:      0,
	}
}

func (g *GeneratorOptions) ShouldCutoff() bool {
	return g.CutoffGenerationSteps > 0 && g.overallComplexity > g.CutoffGenerationSteps
}

func (g *GeneratorOptions) ShouldMinimize() bool {
	return g.MaximumGenerationSteps > 0 && g.overallComplexity > g.MaximumGenerationSteps
}
