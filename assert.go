package chaff

import "fmt"

/**
 * Asserts bounds for a given lower and upper bound
 */
func assertLowerUpperBound(lower int, upper int, lowerName string, upperName string) error {
	if lower == 0 || upper == 0 {
		return nil
	}

	if lower < 0 {
		return fmt.Errorf("%s must be greater than or equal to 0 (lower: %d)", lowerName, lower)
	}

	if upper < 0 {
		return fmt.Errorf("%s must be greater than or equal to 0 (upper: %d)", upperName, upper)
	}

	if lower > upper {
		return fmt.Errorf("%s must be less than or equal to %s (%s: %d, %s: %d)", lowerName, upperName, lowerName, lower, upperName, upper)
	}

	return nil
}

func assertNoUnsupported(node schemaNode) error {
	if node.Not != nil {
		return fmt.Errorf("'not' keyword is not supported")
	}
	if node.If != nil {
		return fmt.Errorf("'if' conditional is not supported")
	}
	if node.Then != nil {
		return fmt.Errorf("'then' conditional is not supported")
	}
	if node.Else != nil {
		return fmt.Errorf("'else' conditional is not supported")
	}
	if len(node.DependentRequired) > 0 {
		return fmt.Errorf("'dependentRequired' is not supported")
	}
	if len(node.DependentSchemas) > 0 {
		return fmt.Errorf("'dependentSchemas' is not supported")
	}
	return nil
}
