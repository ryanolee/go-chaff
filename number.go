package chaff

import (
	"errors"
	"fmt"
	"math"

	"github.com/ryanolee/go-chaff/internal/util"
	"github.com/ryanolee/go-chaff/rand"
)

type (
	numberGeneratorType string
	numberGenerator     struct {
		Type       numberGeneratorType
		Min        float64
		Max        float64
		MultipleOf float64
	}
)

const (
	// Anything smaller than this and Golangs math.Floor and math.Ceil functions
	// begin to misbehave due to floating point precision issues
	infinitesimal = 0.1e-13

	// Bound to slightly less than the max float64 value to avoid unserializable inf values
	upperBound = math.MaxFloat64 / 1000
	lowerBound = -(math.MaxFloat64 / 1000)

	generatorTypeInteger numberGeneratorType = "integer"
	generatorTypeNumber  numberGeneratorType = "number"

	defaultOffset = 10
)

// Parses the "type" keyword of a schema when it is a "number" or "integer"
// Example:
// {
//   "type": "number",
//   "minimum": 0,
//   "maximum": 100
//   "multipleOf": 10
// }

func parseNumber(node schemaNode, genType numberGeneratorType) (Generator, error) {

	min, max, err := resolveMinMaxForNode(node, genType == generatorTypeInteger)
	if err != nil {
		return nil, fmt.Errorf("error parsing number node: %w", err)
	}

	// Return a constant value if min and max are equal
	if min == max {
		return constGenerator{Value: min}, nil
	}

	return &numberGenerator{
		Type:       genType,
		Min:        min,
		Max:        max,
		MultipleOf: util.GetZeroIfNil(node.MultipleOf, 0),
	}, nil
}

func resolveMinMaxForNode(node schemaNode, mustBeAnInteger bool) (float64, float64, error) {
	var min float64 = math.Inf(-1)
	var max float64 = math.Inf(1)

	// Initial Validation
	min = util.GetZeroIfNil(
		util.MaxFloatPtr(node.Minimum, addIfNotNilFloat64(node.ExclusiveMinimum, infinitesimal)),
		math.Inf(-1),
	)

	max = util.GetZeroIfNil(
		util.MinFloatPtr(node.Maximum, addIfNotNilFloat64(node.ExclusiveMaximum, -infinitesimal)),
		math.Inf(1),
	)

	// Set default min and max if they are still infinite
	// Or clamp them to be within a reasonable range of each other
	offset := util.GetFloat(util.GetZeroIfNil(node.MultipleOf, 0)*100, defaultOffset)
	if math.IsInf(min, -1) && math.IsInf(max, 1) {
		min = 0
		max = offset
	} else if math.IsInf(min, -1) {
		min = max - offset
	} else if math.IsInf(max, 1) {
		max = min + offset
	}

	// clamp min and max to be within upper and lower bounds
	if min < lowerBound {
		min = lowerBound
	}

	if max > upperBound {
		max = upperBound
	}

	// If we are an integer type, round min and max to the nearest integers
	if mustBeAnInteger {
		// If min and max are both non-integer values and round to the same integer
		if math.Floor(min) == math.Floor(max) {
			return 0, 0, fmt.Errorf("minimum and maximum do not allow for any integers (min: %f, max: %f)", min, max)
		}

		min = math.Ceil(min)
		max = math.Floor(max)
	}

	// Validate min and max
	if min > max {
		return 0, 0, fmt.Errorf("minimum cannot be greater than maximum (min: %f, max: %f)", min, max)
	}

	// Validate multipleOf
	if node.MultipleOf != nil {
		multipleOf := *node.MultipleOf
		if multipleOf < 0 {
			return 0, 0, errors.New("multipleOf cannot be negative")
		} else if multipleOf != 0 && multipleOf < infinitesimal {
			return 0, 0, fmt.Errorf("multipleOf must be at least %e", infinitesimal)
		} else if mustBeAnInteger && math.Trunc(multipleOf) != multipleOf {
			return 0, 0, errors.New("integer type cannot have a non-integer multipleOf")
		}

		multiplesInRange := countMultiplesInRange(min, max, multipleOf)

		if multiplesInRange == 0 {
			return 0, 0, errors.New("minimum and maximum do not allow for any multiples of multipleOf")
		}
	}
	return min, max, nil
}

func countMultiplesInRange(min float64, max float64, multiple float64) int {
	if min == 0 {
		return int(math.Floor(max / multiple))
	}

	return int(math.Floor(max/multiple)) - int(math.Floor(min/multiple))
}

func generateMultipleOf(rand rand.RandUtil, min float64, max float64, multiple float64) float64 {
	multiplesInRange := countMultiplesInRange(min, max, multiple)

	if multiplesInRange == 0 {
		return 0
	}

	lowerBound := math.Floor(min/multiple) * multiple
	randomMultiple := float64(rand.RandomInt(1, multiplesInRange)) * multiple
	return lowerBound + randomMultiple
}

func roundToInfinitesimal(f float64) float64 {
	return math.Round(f/infinitesimal) * infinitesimal
}

func (g *numberGenerator) Generate(opts *GeneratorOptions) interface{} {
	opts.overallComplexity++
	result := 0.0
	if g.Type == generatorTypeInteger && g.MultipleOf != 0 {
		result = float64(generateMultipleOf(*opts.Rand, g.Min, g.Max, g.MultipleOf))
	} else if g.Type == generatorTypeInteger && g.MultipleOf == 0 {
		result = float64(math.Round(opts.Rand.RandomFloat(g.Min, g.Max)))
	} else if g.Type == generatorTypeNumber && g.MultipleOf != 0 {
		result = util.Round(generateMultipleOf(*opts.Rand, g.Min, g.Max, g.MultipleOf), g.MultipleOf)
	} else if g.Type == generatorTypeNumber && g.MultipleOf == 0 {
		result = opts.Rand.RandomFloat(g.Min, g.Max)
	}

	// Edge case. Make sure 0's are always unsigned
	if math.Abs(result) == 0 {
		return 0
	}

	// The error in calculating multiples can sometimes lead to cases where tiny
	// floating point errors push the result just outside of the min/max bounds.
	return roundToInfinitesimal(result)
}

func (g *numberGenerator) String() string {
	return "NumberGenerator"
}
