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

	// Used to handle the fact that "type" can be a string or an array of strings
	multipleType struct {
		SingleType    string
		MultipleTypes []string
	}

	// Used to handle the fact that "items" can be a schema node or an array of schema nodes
	itemsData struct {
		Node                    *schemaNode
		Nodes                   []schemaNode
		DisallowAdditionalItems bool
	}

	// Used to handle cases where the given value can be a schema node or  a false value
	schemaNodeOrFalse struct {
		Schema  *schemaNode
		IsFalse bool
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

func (s *schemaNodeOrFalse) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if string(data) == "false" {
		s.IsFalse = true
		return nil
	}

	var schema schemaNode
	err := json.Unmarshal(data, &schema)
	if err != nil {
		return err
	}

	s.Schema = &schema
	return nil
}
