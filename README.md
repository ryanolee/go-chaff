# go-chaff
A Json Schema Faker for Go 🙈.

It will generate matching random for a given JSON schema

## Usage
```go
package main
import (
    "github.com/ryanolee/go-chaff"
)

func main() {
    // Parse a schema file
    generator, err := chaff.ParseSchemaFile("{{PathToLocalSchemaFile}}", &chaff.ParserOptions{})

    for key, val := range res.Metadata.Errors {
		fmt.Println("Path: %s Err: %s", key, val)
	}
}
```
