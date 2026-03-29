package chaff

import "fmt"

// WalkSchema recursively visits every JSON object node in a schema tree.
// The visitor is called for each node, parents before children.
// path tracks the JSON pointer path (e.g. "/properties/name").
func walkSchema(node map[string]interface{}, path string, visit func(node map[string]interface{}, path string)) {
	visit(node, path)

	for key, value := range node {
		childPath := path + "/" + key

		if obj, ok := value.(map[string]interface{}); ok {
			walkSchema(obj, childPath, visit)
			continue
		}

		arr, ok := value.([]interface{})
		if !ok {
			continue
		}

		for i, item := range arr {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			walkSchema(obj, fmt.Sprintf("%s/%d", childPath, i), visit)
		}
	}
}
