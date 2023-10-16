package chaff

import (
	"fmt"
	"strings"
)

type (
	referenceGenerator struct {
		ReferenceStr     string
		ReferenceHandler referenceHandler
	}
)

// Parses the "$ref" keyword of a schema
// Example:
// {
//   "$ref": "#/definitions/foo"
// }
func parseReference(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if strings.Contains(node.Ref, "/allOf/") {
		return constGenerator{
			Value: "Invalid Reference containing '/allOf/'",
		}, fmt.Errorf("references to things within allOf are not supported: %s", node.Ref)
	}
	return referenceGenerator{
		ReferenceStr:     node.Ref,
		ReferenceHandler: *metadata.ReferenceHandler,
	}, nil
}

func (g referenceGenerator) Generate(opts *GeneratorOptions) interface{} {
	reference, ok := g.ReferenceHandler.Lookup(g.ReferenceStr)

	if !ok {
		return nil
	}

	refResolver := &opts.ReferenceResolver
	if len(refResolver.GetResolutions()) > opts.MaximumReferenceDepth {
		return fmt.Sprintf("Maximum reference depth exceeded: %d \n %s", opts.MaximumReferenceDepth, refResolver.GetFormattedResolutions())
	}

	if refResolver.HasResolved(g.ReferenceStr) && !opts.BypassCyclicReferenceCheck {
		return fmt.Sprintf("Cyclic reference found: %s \n %s ", refResolver.GetFormattedResolutions(), g.ReferenceStr)
	}

	refResolver.PushRefResolution(g.ReferenceStr)
	defer refResolver.PopRefResolution()

	return reference.Generator.Generate(opts)
}

func (g referenceGenerator) String() string {
	return fmt.Sprintf("ReferenceGenerator{%s}", g.ReferenceStr)
}
