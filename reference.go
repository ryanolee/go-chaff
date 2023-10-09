package chaff

import (
	"fmt"
	"strings"
)

type (
	ReferenceGenerator struct {
		ReferenceStr     string
		ReferenceHandler referenceHandler
	}
)

func parseReference(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if strings.Contains(node.Ref, "/allOf/") {
		return ConstGenerator{
			Value: "Invalid Reference containing '/allOf/'",
		}, fmt.Errorf("references to things within allOf are not supported: %s", node.Ref)
	}
	return ReferenceGenerator{
		ReferenceStr:     node.Ref,
		ReferenceHandler: *metadata.ReferenceHandler,
	}, nil
}

func (g ReferenceGenerator) Generate(opts *GeneratorOptions) interface{} {
	reference, ok := g.ReferenceHandler.Lookup(g.ReferenceStr)
	if !ok {
		return nil
	}

	refResolver := &opts.ReferenceResolver
	if len(refResolver.GetResolutions()) > opts.MaximumReferenceDepth {
		return fmt.Sprintf("Maximum reference depth exceeded: %d \n %s", opts.MaximumReferenceDepth, refResolver.GetFormattedResolutions())
	}

	if !refResolver.HasResolved(g.ReferenceStr) && !opts.BypassCyclicReferenceCheck {
		return fmt.Sprintf("Cyclic reference found: %s \n %s ", refResolver.GetFormattedResolutions(), g.ReferenceStr)
	}

	refResolver.PushRefResolution(g.ReferenceStr)
	defer refResolver.PopRefResolution()

	return reference.Generator.Generate(opts)
}

func (g ReferenceGenerator) String() string {
	return fmt.Sprintf("ReferenceGenerator{%s}", g.ReferenceStr)
}
