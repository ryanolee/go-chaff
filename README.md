# go-chaff
A [Json Schema](https://json-schema.org/) Faker for Go 🙈.

It will generate matching random for a given JSON schema

## Installation
```go get github.com/ryanolee/go-chaff@1```

## Usage
```go
package main
import (
    "github.com/ryanolee/go-chaff"
)

func main() {
    // Parse a schema file
    generator, err := chaff.ParseSchemaFile(`./some/schema/file.json`, &chaff.ParserOptions{})

    for key, val := range res.Metadata.Errors {
		fmt.Println("Path: %s Err: %s", key, val)
	}
}
```


# Current support:
 * `string` (Including `pattern` through (regen)[https://github.com/zach-klippenstein/goregen/blob/master/regen.go] and `formats` through (go faker)[https://github.com/go-faker/faker])
 * `number` and `integer` (Including `multipleOf`)
 * Constant types: `enum`, `const`, `null`
 * References: `$ref`, `$defs`, `definitions`, `$id` 
 * Combination types (`anyOf` / `oneOf` / `allOf`) **N.b These are experimental! Expect none compliant schema output for some of these**
 * Original idea, inspiration and technical support (json-schema-faker )[https://github.com/json-schema-faker/json-schema-faker]

# Credits / Dependencies
 * [Regen](https://github.com/zach-klippenstein/goregen) (@zach-klippenstein and @AnatolyRugalev)
 * [Faker](https://github.com/go-faker/faker)
 * [Schema Store](https://github.com/SchemaStore/schemastore)

# What is left to do?
 * Better test coverage (Property based and unit of various generators)
 * Handle many edge cases where this package might not generate schema compliant results
 * Overcome the limitations of the `oneOf`, `anyOf` and `allOf` keywords implementation in this package.
 * Add support for `if` / `else` keywords

 