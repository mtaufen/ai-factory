package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	sigsyaml "sigs.k8s.io/yaml"
)

// Parse reads a multi-document YAML stream and unmarshals it into the appropriate KRM API types.
// It uses strict struct unmarshaling to ensure no unknown fields exist.
func Parse(r io.Reader) ([]interface{}, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(r, 4096)
	var objects []interface{}

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

		var typeMeta metav1.TypeMeta
		if err := json.Unmarshal(rawObj, &typeMeta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal TypeMeta: %w", err)
		}

		var obj interface{}
		switch typeMeta.Kind {
		case "Run":
			var run Run
			if err := sigsyaml.UnmarshalStrict(rawObj, &run); err != nil {
				return nil, fmt.Errorf("failed to strictly unmarshal Run: %w", err)
			}
			obj = &run
		case "Loop":
			var loop Loop
			if err := sigsyaml.UnmarshalStrict(rawObj, &loop); err != nil {
				return nil, fmt.Errorf("failed to strictly unmarshal Loop: %w", err)
			}
			obj = &loop
		case "Agent":
			var agent Agent
			if err := sigsyaml.UnmarshalStrict(rawObj, &agent); err != nil {
				return nil, fmt.Errorf("failed to strictly unmarshal Agent: %w", err)
			}
			obj = &agent
		case "LocalMCPServer":
			var localMCP LocalMCPServer
			if err := sigsyaml.UnmarshalStrict(rawObj, &localMCP); err != nil {
				return nil, fmt.Errorf("failed to strictly unmarshal LocalMCPServer: %w", err)
			}
			obj = &localMCP
		default:
			return nil, fmt.Errorf("unknown kind: %q", typeMeta.Kind)
		}

		objects = append(objects, obj)
	}

	return objects, nil
}
