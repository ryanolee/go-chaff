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

Circular references by default cause an error (During the generation step) as given above as the generator can recurse infinitely. This can be overridden by passing the `GeneratorOptions` struct to the `Generate` function with the `AllowCircularReferences` set to `true`. The "Complexity" limitation constraint somewhat mitigates this but still can result in infinite recursion hence an overall limit set in config to avoid infinite loops.

## Combinators
Combinators cover `allOf`, `anyOf` and `oneOf`
 * `allOf`  all of uses an a recursive merge algorithm to combine the nodes of a given set of schemas into a single resolvable schemas. References are resolved at compile time and merged down. This approach has the limitation of not being able to handle circular references and in such cases we make accept that the schema is unresolvable and make a best faith attempt to generate a value.
 * `anyOf` and `oneOf` are implemented as a simple random selection of the given schemas. In the case of One of no provision is in place to ensure Exclusivity of the generated values. 

Factoring is supported with the same caveats as above for circular references.

## Complexity
Complexity is managed with two configuration options:
 * `MaximumGenerationSteps`: This limits the number of generation steps that can be taken before "exploration" of the schema is halted. This is a soft limit and generation will continue but with a bias towards simpler structures. (minimal arrays, minimal objects unless forced otherwise by the schema itself)
 * `CutoffGenerationSteps`: This is a hard limit on the number of generation steps that can be taken before generation is instantly halted

Complexity is mesured in "steps" where each step is the recursive traversal to another generator or a retry of a generator due to constraints not being met. With this mechanism in place schemas are normally set to a resonable size balancing generation time and complexity of the generated structures.

It should be noted that MaximumGenerationSteps should be set significantly lower than CutoffGenerationSteps to allow for the soft limit to have an effect.


## Not
The ``not`` combinator works with a three different approaches:
 * **Double negation** in the case of there being more than two nested nots the inner most not is merged into the first node effectively cancelling each other out. The same happens for even numbers of nested nots in an alternating merge
 * **Merging some constraints**: Under most circumstances fields can actually be resolved against the parent node using `notMerge` algorithm where both constraints are merged and reduced down to a single node. This works for most simple constraints such as type, min/max etc.
 * **Brute Force**: If all else fails we brute force some elements of the schema until we find a value that satisfies the `not` constraint and the parent schema. This is only used in cases where there is no nice means of negating the constraints against the `not` ones.