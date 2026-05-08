---
name: loop-api-types-parser
---

Implement a parser utility to read multi-document YAML configurations into `unstructured.Unstructured` objects (from `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured`).
The loader MUST use Kubernetes-native decoders, specifically `k8s.io/apimachinery/pkg/util/yaml.NewYAMLOrJSONDecoder`, to correctly process YAML streams and decode them into generic unstructured KRM resources.
Include unit tests in `parser_test.go` using YAML snippets from the spec/ideas to verify correct parsing of `Run`, `Loop`, `Agent`, and `LocalMCPServer` objects into unstructured form.
