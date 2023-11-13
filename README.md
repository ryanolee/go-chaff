# go-chaff
A [Json Schema](https://json-schema.org/) Faker for Go 🙈.

It will generate matching random for a given JSON schema

<img src='docs/images/logo.png' width='350'>

> [CC0 by @Iroshi_]
## Documentation
Documentation for the library functions can be [found here](https://pkg.go.dev/github.com/ryanolee/go-chaff).

## Installation
```go get github.com/ryanolee/go-chaff@1```

## Usage
```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/ryanolee/go-chaff"
)

const schema = `{"type": "number"}`

func main() {
	generator, err := chaff.ParseSchemaStringWithDefaults(schema)
	if err != nil {
		panic(err)
	}
	
	fmt.Println(generator.Metadata.Errors)
	result := generator.GenerateWithDefaults()
	if err != nil {
		panic(err)
	}

	res, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(res))
}
```

# CLI
There is also a cli tool available for this package that can be installed from the [releases](https://github.com/ryanolee/go-chaff/releases) section.
```
Usage: go-chaff [flags]
  -file string
        Specify a file path to read the JSON Schema from
  -format
        Format JSON output.
  -help
        Print out help.
  -output string
        Specify file path to write generated output to.
  -verbose
        Print out detailed error information.
  -version
        Print out cli version information.
```

You can also pipe into STDIN for the cli
```bash
echo '{"type": "string", "format": "ipv4"}' | go-chaff
"217.2.244.95"
```

# Current support:
 * Strings: (Including `pattern` through [regen](https://github.com/zach-klippenstein/goregen/blob/master/regen.go) and `formats` through [go faker](https://github.com/go-faker/faker))
 * Number / Integer: `multipleOf`, `min`, `max`, `exclusiveMin`, `exclusiveMax`
 * Constant types: `enum`, `const`, `null`
 * References: `$ref`, `$defs`, `definitions`, `$id` 
 * Object: `properties`, `patternProperties`, `additionalProperties`, `minProperties`, `maxProperties`, `required`
 * Array: `items`, `minItems`, `maxItems`, `contains`, `minContains`, `maxContains`, `prefixItems`, `additionalItems`
 * Combination types (`anyOf` / `oneOf` / `allOf`) **N.b These are experimental! Expect none compliant schema output for some of these**

# Credits / Dependencies
 * [Regen](https://github.com/zach-klippenstein/goregen) (@zach-klippenstein and @AnatolyRugalev)
 * [Faker](https://github.com/go-faker/faker)
 * [Schema Store](https://github.com/SchemaStore/schemastore)
 * Original idea, inspiration and technical support [json-schema-faker](https://github.com/json-schema-faker/json-schema-faker)

# What is left to do?
 * Better test coverage (Property based and unit of various generators)
 * Handle many edge cases where this package might not generate schema compliant results
 * Overcome the limitations of the current `oneOf`, `anyOf` and `allOf` keywords implementation.
 * Add support for `if` / `else` keywords

 
