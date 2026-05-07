---
name: loop-api-types-structs
---

Create the core Go structs for the AI Factory Loop Execution Engine (`Run`, `Loop`, `Agent`, `LocalMCPServer`) as defined in the `loop-api-types` spec.
Place these in the `factory/pkg/runtime/api` package. Ensure that the structs include the `//+k8s:deepcopy-gen=true` and `//+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object` annotations to allow standard Kubernetes tooling to generate `DeepCopy` methods.
Include standard KRM `TypeMeta` and `ObjectMeta` fields, and use JSON struct tags as prescribed in the spec.
Also provide a `doc.go` for the package.
