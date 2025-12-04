package chaff

import (
	"encoding/json"
)

type (
	// Used to handle the fact that "type" can be a string or an array of strings
	multipleType struct {
		SingleType    string
		MultipleTypes []string
	}

	// Used to handle the fact that "items" can be a schema node or an array of schema nodes
	itemsData struct {
		Node                    *schemaNode
		Nodes                   *[]schemaNode
		DisallowAdditionalItems bool
	}

	// Used to handle cases where the given value can be a schema node or  a false value
	schemaNodeOrFalse struct {
		Schema  *schemaNode
		IsFalse bool
	}
)

func newMultipleTypeFromSlice(types []string) multipleType {
	multipleType := multipleType{}
	if len(types) == 1 {
		multipleType.SingleType = types[0]
	} else if len(types) > 1 {
		multipleType.MultipleTypes = types
	}

	return multipleType
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

func (m *multipleType) MarshalJSON() ([]byte, error) {
	if m.SingleType != "" {
		return json.Marshal(m.SingleType)
	} else if len(m.MultipleTypes) > 0 {
		return json.Marshal(m.MultipleTypes)
	}

	return []byte("null"), nil
}

func (i *itemsData) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if string(data) == "false" {
		i.DisallowAdditionalItems = true
		return nil
	}

	var nodes *[]schemaNode
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

func (i *itemsData) MarshalJSON() ([]byte, error) {
	if i.DisallowAdditionalItems {
		return []byte("false"), nil
	}

	if i.Node != nil {
		return json.Marshal(i.Node)
	} else if i.Nodes != nil {
		return json.Marshal(i.Nodes)
	}

	return []byte("null"), nil
}

func (s *schemaNodeOrFalse) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if string(data) == "false" {
		s.IsFalse = true
		return nil
	}

	if string(data) == "true" {
		s.IsFalse = false
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

func (s *schemaNodeOrFalse) MarshalJSON() ([]byte, error) {
	if s.IsFalse {
		return []byte("false"), nil
	}

	if s.Schema != nil {
		return json.Marshal(s.Schema)
	}

	return []byte("null"), nil
}
