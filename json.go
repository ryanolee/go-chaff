package chaff

import (
	"encoding/json"
)

type (
	// Additional properties can be a schema node or a boolean value.
	// This handles both cases.
	additionalData struct {
		Schema             *schemaNode
		DisallowAdditional bool
	}

	multipleType struct {
		SingleType    string
		MultipleTypes []string
	}

	itemsData struct {
		Node                    *schemaNode
		Nodes                   []schemaNode
		DisallowAdditionalItems bool
	}
)

func (a *additionalData) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if string(data) == "false" {
		a.DisallowAdditional = true
		return nil
	}

	if string(data) == "true" {
		a.DisallowAdditional = false
		return nil
	}

	var schema schemaNode
	err := json.Unmarshal(data, &schema)
	if err != nil {
		return err
	}

	a.Schema = &schema
	return nil
}

func (m *multipleType) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	var multipleTypes []string
	var singleType string

	// Try to parse an array of types
	multipleTypesError := json.Unmarshal(data, &multipleTypes)
	singleTypeError := json.Unmarshal(data, &singleType)

	if multipleTypesError != nil && singleTypeError != nil {
		return singleTypeError
	}

	m.MultipleTypes = multipleTypes
	m.SingleType = singleType

	return nil
}

func (i *itemsData) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if string(data) == "false" {
		i.DisallowAdditionalItems = true
		return nil
	}

	var nodes []schemaNode
	var node *schemaNode
	nodeErr := json.Unmarshal(data, &node)
	nodesErr := json.Unmarshal(data, &nodes)
	if nodeErr != nil && nodesErr != nil {
		return nodeErr
	}

	i.Nodes = nodes
	i.Node = node
	return nil
}
