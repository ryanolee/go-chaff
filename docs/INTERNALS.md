# Internal Project Structure
This document aims to give some context on how the project works internally for anyone who wants to contribute to the project.

## High level overview
Usage for the library is set into two distinct parts:
 * `Parsing`: Covers reading a JSON Schema file, Validating <em>some</em> elements of the file structure and converting it into a tree of `Generators`.
 * `Generating`: Based on the constraints set during the `Parsing` step generators can be called recursively to generate a random Struct that matches the schema.

## Parsing
The json schema is unmarshaled into a tree of `SchemaNode` structs. These structs are used to validate the schema and create a tree of `Generators` which can be used to create a constant stream of random values.

## Generators
Below is an annoteted example of a `Generator` for a `const` type. This is the simplest type of generator and is used to generate a constant value.
```go
package chaff

import "fmt"

// All generators implement the Generator interface and are
type (
	ConstGenerator struct {
        // We need to store the value of the constant which can be stored in the generator as an interface{}
		Value interface{}
	}
)

// All generators have an associated Parse function which is used to create a generator from a "SchemaNode"
func parseConst(node schemaNode) (ConstGenerator, error) {
	return ConstGenerator{
		Value: node.Const,
	}, nil
}

// All generators implement the Generate function which is used to generate a random value based on the constraints of the generator
func (g ConstGenerator) Generate(opts *GeneratorOptions) interface{} {
	return g.Value
}

// All generators implement the Stringer interface which is used to print the generator in a human readable format
func (g ConstGenerator) String() string {
	return fmt.Sprintf("ConstGenerator[%s]", g.Value)
}
```

## References
References are created over the course of the initial "parse" traversal. Every "referenceable" item is stored in am internal map of "References". When generating a value for a reference the generator will look up the reference in the map and call the `Generate` function on the referenced generator.

Circular references by default cause an error (During the generation step) as given above as the generator can recurse infinitely. This can be overridden by passing the `GeneratorOptions` struct to the `Generate` function with the `AllowCircularReferences` set to `true`.

