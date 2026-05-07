---
name: loop-api-types-parser
---

Implement a parser utility to read multi-document YAML configurations into the structs defined in the `loop-api-types-structs` task.
The loader MUST use Kubernetes-native decoders, specifically `k8s.io/apimachinery/pkg/util/yaml.NewYAMLOrJSONDecoder`, to correctly process YAML streams and unmarshal into the strict struct definitions.
Include unit tests in `parser_test.go` using YAML snippets from the spec/ideas to verify correct parsing of `Run`, `Loop`, `Agent`, and `LocalMCPServer` objects.
