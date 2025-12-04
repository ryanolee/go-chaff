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

// MinFloatPtr returns the lowest value float pointer
func MinFloatPtr(a ...*float64) *float64 {
	var min *float64
	for _, v := range a {
		if v != nil {
			if min == nil || *v < *min {
				min = v
			}
		}
	}

	return min
}

func MaxFloatPtr(a ...*float64) *float64 {
	var max *float64
	for _, v := range a {
		if v != nil {
			if max == nil || *v > *max {
				max = v
			}
		}
	}

	return max
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

func GetIndexOrDefault[T any](nodes []T, index int, defaultItem T) T {
	if index < 0 || index >= len(nodes) {
		return defaultItem
	}

	return nodes[index]
}

// GetObjectKeyOrDefault returns the value for the given key in the map
// or the defaultItem if the key does not exist or the map itself is nil
func GetObjectKeyOrDefault[T any](obj *map[string]T, key string, defaultItem T) T {
	if obj == nil {
		return defaultItem
	}

	if val, ok := (*obj)[key]; ok {
		return val
	}

	return defaultItem
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

func SafeMarshalListToJsonList(data *[]interface{}) []string {
	if data == nil {
		return []string{}
	}

	result := []string{}
	for _, item := range *data {
		result = append(result, MarshalJsonToString(item))
	}

	return result
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

// Extract the keys from a map[string]T as a []string
func MapKeysToStringSlice[T any](maps ...*map[string]T) []string {
	result := []string{}
	for _, m := range maps {
		if m == nil {
			continue
		}

		for key, _ := range *m {
			result = append(result, key)
		}
	}

	return result
}
