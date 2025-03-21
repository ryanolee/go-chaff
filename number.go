package chaff

import (
	"errors"
	"math"

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
	infinitesimal                            = math.SmallestNonzeroFloat64
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
	var min float64
	var max float64

	// Initial Validation
	if node.Minimum != 0 && node.ExclusiveMinimum != 0 {
		return nullGenerator{}, errors.New("cannot have both minimum and exclusive minimum")
	}

	if node.Maximum != 0 && node.ExclusiveMaximum != 0 {
		return nullGenerator{}, errors.New("cannot have both maximum and exclusive maximum")
	}

	// Set min and max
	if node.Minimum != 0 {
		min = float64(node.Minimum)
	} else if node.ExclusiveMinimum != 0 {
		min = float64(node.ExclusiveMinimum) + infinitesimal
	}

	if node.Maximum != 0 {
		max = float64(node.Maximum)
	} else if node.ExclusiveMaximum != 0 {
		max = float64(node.ExclusiveMaximum) - infinitesimal
	} else if min != 0 {
		max = min + defaultOffset
	} else {
		max = defaultOffset
	}

	// Validate min and max
	if min > max {
		return nullGenerator{}, errors.New("minimum cannot be greater than maximum")
	}

	// Validate multipleOf
	if node.MultipleOf != 0 {
		if node.MultipleOf <= 0 {
			return nullGenerator{}, errors.New("multipleOf cannot be negative or zero")
		}

		multiplesInRange := countMultiplesInRange(min, max, node.MultipleOf)

		if multiplesInRange == 0 {
			return nullGenerator{}, errors.New("minimum and maximum do not allow for any multiples of multipleOf")
		}
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
		return generateMultipleOf(*opts.Rand, g.Min, g.Max, g.MultipleOf)
	} else if g.Type == generatorTypeNumber && g.MultipleOf == 0 {
		return opts.Rand.RandomFloat(g.Min, g.Max)
	}

	return 0
}

func (g *numberGenerator) String() string {
	return "NumberGenerator"
}
