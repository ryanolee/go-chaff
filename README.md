# go-chaff
A [Json Schema](https://json-schema.org/) Faker for Go 🙈.

It will genreate random data that _should_ validate against a given schema.

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
CLI too for generating random JSON data matching given JSON schema
Usage: go-chaff [flags]
  -allow-insecure
        Allow fetching remote $ref documents over insecure HTTP connections.
  -allow-outside-cwd
        Allow fetching $ref documents from file system paths outside the current working directory.
  -allowed-hosts string
        Comma separated list of allowed hosts to fetch remote $ref documents from over HTTP(S). If empty http and https resolution will fail.
  -allowed-paths string
        Comma separated list of allowed file system paths to fetch $ref documents from.
  -bypass-cyclic-reference-check
        Bypass cyclic reference check when generating schemas with cyclic $ref references.
  -cutoff-generation-steps int
        Maximum number of generation steps to perform before aborting generation entirely and returning what was generated. (default 2000)
  -file string
        Specify a file path to read the JSON Schema from
  -format
        Format JSON output.
  -help
        Print out help.
  -maximum-generation-steps int
        Maximum number of generation steps to perform before reducing the effort put into the generation process to a bare minimum. (default 1000)
  -maximum-if-attempts int
        Maximum number of attempts to satisfy 'if' conditions when generating data. (default 100)
  -maximum-oneof-attempts int
        Maximum number of attempts to satisfy 'oneOf' conditions when generating data. (default 100)
  -maximum-reference-depth int
        Maximum depth of $ref references to resolve at once when generating data. (default 10)
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
 * Strings: (Including `pattern` through [regen](https://github.com/zach-klippenstein/goregen/blob/master/regen.go) and `formats` through [go faker](https://github.com/go-faker/faker)), `minLength`, `maxLength` 
 * Number / Integer: `multipleOf`, `min`, `max`, `exclusiveMin`, `exclusiveMax`
 * Constant types: `enum`, `const`, `null`
 * References: `$ref`, `$defs`, `definitions`, `$id` 
 * Object: `properties`, `patternProperties`, `additionalProperties`, `minProperties`, `maxProperties`, `required`
 * Array: `items`, `minItems`, `maxItems`, `contains`, `minContains`, `maxContains`, `prefixItems`, `additionalItems`, `unevaluatedItems`, `uniqueItems` (Limited support)
 * Combination types `anyOf` / `oneOf` / `allOf` 
 * Support for `if` / `then` / `else` 
 * Support for `not` combinator (excluding `anyOf`, `oneOf` / `allOf` and `if/then/else`)
 * Multi document resolution for `$ref` over  `http(s)` or `file` schemes.

# Credits / Dependencies
 * [Regen](https://github.com/zach-klippenstein/goregen) (@zach-klippenstein and @AnatolyRugalev)
 * [Faker](https://github.com/go-faker/faker)
 * [Schema Store](https://github.com/SchemaStore/schemastore)
 * [jsonschema](github.com/kaptinlin/jsonschema) from @kaptinlin for test validation
 * [jsonschema](https://github.com/santhosh-tekuri/jsonschema) from @santhosh-tekuri for internal schema constrain validation
 * Original idea, inspiration and technical support [json-schema-faker](https://github.com/json-schema-faker/json-schema-faker)

# What is left to do?
 * Better test coverage (Property based and unit of various generators)
 * Handle many edge cases where this package might not generate schema compliant results
 * Overcome the limitations of the current `oneOf`, `anyOf` and `allOf` keywords implementation.

 
