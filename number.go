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
	var min float64 = math.Inf(-1)
	var max float64 = math.Inf(1)

	// Initial Validation
	if node.Minimum != nil && node.ExclusiveMinimum != nil {
		return nullGenerator{}, errors.New("cannot have both minimum and exclusive minimum")
	}

	if node.Maximum != nil && node.ExclusiveMaximum != nil {
		return nullGenerator{}, errors.New("cannot have both maximum and exclusive maximum")
	}

	// Set min and max
	if node.Minimum != nil {
		min = float64(*node.Minimum)
	} else if node.ExclusiveMinimum != nil {
		min = float64(*node.ExclusiveMinimum) + infinitesimal
	}

	// Set maximum
	if node.Maximum != nil {
		max = float64(*node.Maximum)
	} else if node.ExclusiveMaximum != nil {
		max = float64(*node.ExclusiveMaximum) - infinitesimal
	}

	// Set default min and max if they are still infinite
	// Or clamp them to be within a reasonable range of each other
	offset := util.GetFloat(node.MultipleOf*100, defaultOffset)
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
	if genType == generatorTypeInteger {
		if math.Abs(min-max) < 1 {
			return nullGenerator{}, fmt.Errorf("minimum and maximum do not allow for any integers (min: %f, max: %f)", min, max)
		}

		min = math.Ceil(min)
		max = math.Floor(max)
	}

	// Validate min and max
	if min > max {
		return nullGenerator{}, fmt.Errorf("minimum cannot be greater than maximum (min: %f, max: %f)", min, max)
	}

	// Validate multipleOf
	if node.MultipleOf != 0 {
		if node.MultipleOf < 0 {
			return nullGenerator{}, errors.New("multipleOf cannot be negative")
		} else if node.MultipleOf != 0 && node.MultipleOf < infinitesimal {
			return nullGenerator{}, fmt.Errorf("multipleOf must be at least %e", infinitesimal)
		} else if genType == generatorTypeInteger && math.Trunc(node.MultipleOf) != node.MultipleOf {
			return nullGenerator{}, errors.New("integer type cannot have a non-integer multipleOf")
		}

		multiplesInRange := countMultiplesInRange(min, max, node.MultipleOf)

		if multiplesInRange == 0 {
			return nullGenerator{}, errors.New("minimum and maximum do not allow for any multiples of multipleOf")
		}
	}

	// Return a constant value if min and max are equal
	if min == max {
		return constGenerator{Value: min}, nil
	}

	return &numberGenerator{
		Type:       genType,
		Min:        min,
		Max:        max,
		MultipleOf: node.MultipleOf,
	}, nil
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
func (g *numberGenerator) Generate(opts *GeneratorOptions) interface{} {
	opts.overallComplexity++
	if g.Type == generatorTypeInteger && g.MultipleOf != 0 {
		return int(generateMultipleOf(*opts.Rand, g.Min, g.Max, g.MultipleOf))
	} else if g.Type == generatorTypeInteger && g.MultipleOf == 0 {
		return int(math.Round(opts.Rand.RandomFloat(g.Min, g.Max)))
	} else if g.Type == generatorTypeNumber && g.MultipleOf != 0 {
		return util.Round(generateMultipleOf(*opts.Rand, g.Min, g.Max, g.MultipleOf), g.MultipleOf)
	} else if g.Type == generatorTypeNumber && g.MultipleOf == 0 {
		return opts.Rand.RandomFloat(g.Min, g.Max)
	}

	return 0
}

func (g *numberGenerator) String() string {
	return "NumberGenerator"
}
