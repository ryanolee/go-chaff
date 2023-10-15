package rand

import (
	"math/rand"
	"time"

	"github.com/thoas/go-funk"
)

type RandUtil struct {
	Source rand.Source
	Rand   *rand.Rand
}

func NewRandUtilFromString(stringSeed string) *RandUtil {
	seed := int64(0)
	for i, c := range stringSeed {
		seed += int64(c) * int64(i+1)
	}
	return NewRandUtil(seed)
}

func NewRandUtilFromTime() *RandUtil {
	return NewRandUtil(time.Now().UnixNano())
}

func NewRandUtil(seed int64) *RandUtil {
	source := rand.NewSource(seed)
	r := rand.New(source)
	return &RandUtil{
		Source: source,
		Rand:   r,
	}
}

// Generic functions
func (sr *RandUtil) Choice(slice []interface{}) interface{} {
	return slice[sr.Rand.Intn(len(slice))]
}

// Array functions
func (sr *RandUtil) StringChoice(stringSlice *[]string) string {
	return (*stringSlice)[sr.Rand.Intn(len(*stringSlice))]
}

func (sr *RandUtil) StringChoiceMultiple(stringSlice *[]string, numChoices int) []string {
	// Pick NumChoices random choices from the string slice without duplicates
	choices := funk.Shuffle(*stringSlice).([]string)

	return choices[:numChoices]

}

// Int functions
func (sr *RandUtil) RandomInt(min int, max int) int {
	// In the the case that min == max, return min
	if min == max {
		return min
	}

	// Random int supporting negative numbers
	return sr.Rand.Intn(max-min) + min
}

// Float functions
func (sr *RandUtil) RandomFloat(min float64, max float64) float64 {
	return sr.Rand.Float64()*(max-min) + min
}

// Bool functions
func (sr *RandUtil) RandomBool() bool {
	return sr.Rand.Intn(2) == 1
}
