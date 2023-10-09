package chaff

import (
	"fmt"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/ryanolee/go-chaff/internal/regen"
)

type (
	stringGenerator struct {
		Format           stringFormat
		Pattern          string
		PatternGenerator regen.Generator
	}
)

type stringFormat string

const (
	// Time
	formatDateTime stringFormat = "date-time" // RFC3339
	formatTime     stringFormat = "time"      //
	formatDate     stringFormat = "date"
	formatDuration stringFormat = "duration"

	// Email
	formatEmail    stringFormat = "email"
	formatIdnEmail stringFormat = "idn-email"

	// Hostname
	formatHostname    stringFormat = "hostname"
	formatIdnHostname stringFormat = "idn-hostname"

	// IP
	formatIpv4 stringFormat = "ipv4"
	formatIpv6 stringFormat = "ipv6"

	// Rescource Identifier
	formatUUID         stringFormat = "uuid"
	formatURI          stringFormat = "uri"
	formatURIReference stringFormat = "uri-reference"
	formatIRI          stringFormat = "iri"
	formatIRIReference stringFormat = "iri-reference"

	// Uri Template
	formatUriTemplate stringFormat = "uri-template"

	// JSON Pointer
	formatJSONPointer         stringFormat = "json-pointer"
	formatRelativeJSONPointer stringFormat = "relative-json-pointer"

	// Regex
	formatRegex stringFormat = "regex"
)

func parseString(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if node.Format != "" && node.Pattern != "" {
		return NullGenerator{}, fmt.Errorf("cannot have both format and pattern on a string")
	}

	generator := stringGenerator{
		Format:  stringFormat(node.Format),
		Pattern: node.Pattern,
	}

	if node.Pattern != "" {
		regenGenerator, err := newRegexGenerator(node.Pattern, metadata.ParserOptions.RegexStringOptions)
		if err != nil {
			return NullGenerator{}, fmt.Errorf("invalid regex pattern: %s", node.Pattern)
		}

		generator.PatternGenerator = regenGenerator
	}

	return generator, nil
}

func (g stringGenerator) Generate(opts *GeneratorOptions) interface{} {
	if g.Pattern != "" {
		return g.PatternGenerator.Generate()
	}

	if g.Format != "" {
		return generateFormat(g.Format, opts)
	}

	return faker.Sentence()
}

func (g stringGenerator) String() string {
	return "StringGenerator"
}


func generateFormat(format stringFormat, opts *GeneratorOptions) string {
	switch format {
	case formatDateTime:
		return time.Unix(faker.UnixTime(), 0).Format(time.RFC3339)
	case formatTime:
		return fmt.Sprintf("%s+00:00", time.Unix(faker.UnixTime(), 0).Format(time.TimeOnly))
	case formatDate:
		return time.Unix(faker.UnixTime(), 0).Format(time.DateOnly)
	case formatDuration:
		return fmt.Sprintf("P%dD", opts.Rand.RandomInt(0, 90))
	case formatEmail, formatIdnEmail:
		return faker.Email()
	case formatHostname, formatIdnHostname:
		return faker.DomainName()
	case formatIpv4:
		return faker.IPv4()
	case formatIpv6:
		return faker.IPv6()
	case formatUUID:
		return faker.UUIDHyphenated()
	case formatURI, formatURIReference, formatIRI, formatIRIReference:
		return faker.URL()
	case formatUriTemplate, formatJSONPointer, formatRelativeJSONPointer, formatRegex:
		return fmt.Sprintf("Known but unsupported format: %s", format)
	default:
		return fmt.Sprintf("Unsupported Format: %s", format)
	}
}
