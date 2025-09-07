package util

import "math"

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

func FloatPtr(f float64) *float64 {
	return &f
}
