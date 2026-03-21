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
		Rand *rand.RandUtil `json:"-"`

		// The default minimum number value
		DefaultNumberMinimum int `json:"defaultNumberMinimum,omitempty" jsonschema:"title=Default Number Minimum"`

		// The default maximum number value
		DefaultNumberMaximum int `json:"defaultNumberMaximum,omitempty" jsonschema:"title=Default Number Maximum"`

		// The default minimum String length
		DefaultStringMinLength int `json:"defaultStringMinLength,omitempty" jsonschema:"title=Default String Minimum Length"`

		// The default maximum String length
		DefaultStringMaxLength int `json:"defaultStringMaxLength,omitempty" jsonschema:"title=Default String Maximum Length"`

		// The default minimum array length
		DefaultArrayMinItems int `json:"defaultArrayMinItems,omitempty" jsonschema:"title=Default Array Minimum Items"`

		// The default maximum array length
		// This will be set min + this inf the event a minimum value is set
		DefaultArrayMaxItems int `json:"defaultArrayMaxItems,omitempty" jsonschema:"title=Default Array Maximum Items"`

		// The default minimum object properties (Will be ignored if there are fewer properties available)
		DefaultObjectMinProperties int `json:"defaultObjectMinProperties,omitempty" jsonschema:"title=Default Object Minimum Properties"`

		// The default maximum object properties (Will be ignored if there are fewer properties available)
		DefaultObjectMaxProperties int `json:"defaultObjectMaxProperties,omitempty" jsonschema:"title=Default Object Maximum Properties"`

		// The maximum dnumber of references to resolve at once in terms of depth (Default: 10)
		MaximumReferenceDepth int `json:"maximumReferenceDepth,omitempty" jsonschema:"title=Maximum Reference Depth"`

		// In the event that schemas are recursive there is a good chance the generator
		// can run forever. This option will bypass the check for cyclic references
		// Please defer to the MaximumReferenceDepth option if possible when using this
		BypassCyclicReferenceCheck bool `json:"bypassCyclicReferenceCheck,omitempty" jsonschema:"title=Bypass Cyclic Reference Check"`

		// Used to keep track of references during a resolution cycle (Used internally and can be ignored)
		ReferenceResolver referenceResolver `json:"-"`

		// Though technically in some cases a schema may allow for additional
		// values it might not always be desireable. this option suppresses fallback_n values
		// so that they will only appear to make up a "minimum value" forces them to
		SuppressFallbackValues bool `json:"suppressFallbackValues,omitempty" jsonschema:"title=Suppress Fallback Values"`

		// The maximum number of times to attempt to generate a unique value
		// when using unique* constraints in schemas
		MaximumUniqueGeneratorAttempts int `json:"maximumUniqueGeneratorAttempts,omitempty" jsonschema:"title=Maximum Unique Generator Attempts"`

		// The maximum number of times to attempt to satisfy "if" statements
		// before giving up
		MaximumIfAttempts int `json:"maximumIfAttempts,omitempty" jsonschema:"title=Maximum If Attempts"`

		// The maximum number of times to attempt to satisfy "oneOf" statements
		// before giving up
		MaximumOneOfAttempts int `json:"maximumOneOfAttempts,omitempty" jsonschema:"title=Maximum OneOf Attempts"`

		// The maximum number of steps to take when generating a value
		// after which the the generator will begin to do the "bare minimum" to generate a value
		MaximumGenerationSteps int `json:"maximumGenerationSteps,omitempty" jsonschema:"title=Maximum Generation Steps"`

		// The maximum number of steps to take before giving up entirely and aborting generation
		// This is a hard cap on generation steps to prevent extremely long generation times
		CutoffGenerationSteps int `json:"cutoffGenerationSteps,omitempty" jsonschema:"title=Cutoff Generation Steps"`

		overallComplexity int `json:"-"`
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
