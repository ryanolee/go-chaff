package chaff

import (
	"fmt"
	"strings"
)

type (
	referenceGenerator struct {
		// Document the reference points to
		Document string

		// The reference string (e.g. "#/definitions/foo")
		ReferenceStr string

		// The handler that contains all parsed references
		ReferenceHandler referenceHandler
	}
)

// Parses the "$ref" keyword of a schema
// Example:
//
//	{
//	  "$ref": "#/definitions/foo"
//	}
func parseReference(node schemaNode, metadata *parserMetadata) (Generator, error) {
	if node.Ref == nil {
		return nullGenerator{}, fmt.Errorf("reference node missing $ref property")
	}

	if strings.Contains(*node.Ref, "/allOf/") {
		return nil, fmt.Errorf("references to things within allOf are not supported: %s", *node.Ref)
	}

	documentId, ref, err := metadata.DocumentResolver.HandleDeferredReferenceResolution(*node.Ref, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to handle deferred reference resolution for ref '%s': %w", *node.Ref, err)
	}

	return referenceGenerator{
		Document:         documentId,
		ReferenceStr:     ref,
		ReferenceHandler: *metadata.ReferenceHandler,
	}, nil
}

func (g referenceGenerator) Generate(opts *GeneratorOptions) interface{} {
	opts.overallComplexity++
	reference, ok := g.ReferenceHandler.Lookup(g.Document, g.ReferenceStr)

	if !ok {
		return nil
	}

	refResolver := &opts.ReferenceResolver
	if len(refResolver.GetResolutions()) > opts.MaximumReferenceDepth {
		return fmt.Sprintf("Maximum reference depth exceeded: %d \n %s", opts.MaximumReferenceDepth, refResolver.GetFormattedResolutions())
	}

	if refResolver.HasResolved(g.Document, g.ReferenceStr) && !opts.BypassCyclicReferenceCheck {
		return fmt.Sprintf("Cyclic reference found: %s \n %s ", refResolver.GetFormattedResolutions(), g.ReferenceStr)
	}

	refResolver.PushRefResolution(g.Document, g.ReferenceStr)
	defer refResolver.PopRefResolution()

	return reference.Generator.Generate(opts)
}

func (g referenceGenerator) String() string {
	return fmt.Sprintf("ReferenceGenerator{document: %s, path: %s}", g.Document, g.ReferenceStr)
}
