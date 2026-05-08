package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Parse reads a multi-document YAML stream and unmarshals it into generic unstructured KRM resources.
func Parse(r io.Reader) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(r, 4096)
	var objects []*unstructured.Unstructured

	for {
		var rawObj json.RawMessage
		err := decoder.Decode(&rawObj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode document: %w", err)
		}

		// Skip empty documents
		if len(rawObj) == 0 || bytes.Equal(bytes.TrimSpace(rawObj), []byte("null")) {
			continue
		}

		u := &unstructured.Unstructured{}
		if err := json.Unmarshal(rawObj, u); err != nil {
			return nil, fmt.Errorf("failed to unmarshal unstructured object: %w", err)
		}

		objects = append(objects, u)
	}

	return objects, nil
}
