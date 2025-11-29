package chaff

import (
	"fmt"
)

type (
	errorCollection struct {
		referenceHandler *referenceHandler
		documentResolver *documentResolver
		Errors           map[string]map[string]error
	}
)

func newErrorCollection(
	referenceHandler *referenceHandler,
	documentResolver *documentResolver,
) *errorCollection {
	return &errorCollection{
		referenceHandler: referenceHandler,
		documentResolver: documentResolver,
		Errors:           make(map[string]map[string]error),
	}
}

func (ec *errorCollection) AddErrorWithSubpath(subPath string, err error) {
	document, path := ec.documentResolver.GetDocumentIdCurrentlyBeingParsed(), ec.referenceHandler.CurrentPath

	if _, exists := ec.Errors[document]; !exists {
		ec.Errors[document] = make(map[string]error)
	}

	ec.Errors[document][path+subPath] = err
}

func (ec *errorCollection) AddError(err error) {
	ec.AddErrorWithSubpath("", err)
}

func (ec *errorCollection) HasErrors() bool {
	return len(ec.Errors) > 0
}

func (ec *errorCollection) CollectErrors() map[string]error {
	flattenedErrors := make(map[string]error)

	for doc, docErrors := range ec.Errors {
		for path, err := range docErrors {
			flattenedErrors[fmt.Sprintf("%s -> %s", doc, path)] = err
		}
	}

	return flattenedErrors
}
