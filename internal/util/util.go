package util

import (
	"encoding/json"
	"math"
	"regexp"
	"strings"

	"github.com/thoas/go-funk"
)

// Returns the first non-empty string
func GetString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

// Returns the first non-zero int
func GetInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}

	return 0
}

// Returns the first non-zero float
func GetFloat(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}

	return 0
}

func GetFloatPtr(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}

	return nil
}

func GetPtr[T any](values ...*T) *T {
	for _, value := range values {
		if value != nil {
			return value
		}
	}

	return nil
}

func GetValue(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}

	return nil
}

func MergeSlicePtrs[T any](slices ...*[]T) *[]T {
	var result []T

	for _, slice := range slices {
		if slice != nil {
			result = append(result, *slice...)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return &result
}

func AnyNotNil(values ...interface{}) bool {
	for _, value := range values {
		if value != nil {
			return true
		}
	}

	return false
}

func Round(x, unit float64) float64 {
	return math.Round(x/unit) * unit
}

// Returns the true if value or defaultValue are true
func GetBool(value bool, defaultValue bool) bool {
	if !value {
		return defaultValue
	}

	return value
}

// MaxInt returns the highest value int in a variadic list of ints
func MaxInt(a ...int) int {
	max := a[0]
	for _, v := range a {
		if v > max {
			max = v
		}
	}

	return max
}

// MinInt returns the lowest value int in a variadic list of ints
func MinInt(a ...int) int {
	min := a[0]
	for _, v := range a {
		if v < min {
			min = v
		}
	}

	return min
}

func GetZeroIfNil[T any](f *T, zeroValue T) T {
	if f == nil {
		return zeroValue
	}

	return *f
}

func ImplodeMapStrings[V any](mapKeys map[string]V) string {
	keys, ok := funk.Keys(mapKeys).([]string)
	if !ok {
		return ""
	}

	return strings.Join(keys, ",")

}

// Marshal Data to a string
// in error cases the string is empty and the error is blackholed
func MarshalJsonToString(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}

	return string(jsonData)
}

// Unmarshal a string to a map
func UnmarshalJsonStringToMap(data string) interface{} {
	var result interface{}
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return nil
	}

	return result
}

// Regex to named capture groups
func RegexMatchNamedCaptureGroups(r *regexp.Regexp, str string) map[string]string {
	matches := r.FindStringSubmatch(str)
	results := map[string]string{}
	for i, name := range r.SubexpNames() {
		if i != 0 && name != "" {
			results[name] = matches[i]
		}
	}

	return results
}
