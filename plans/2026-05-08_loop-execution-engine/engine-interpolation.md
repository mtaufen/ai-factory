---
name: engine-interpolation
---

This task implements the variable interpolation logic for step and loop arguments.
Create `factory/pkg/runtime/engine/interpolate.go` with a function like `InterpolateArgs(args []api.Argument, context map[string]string) map[string]string`.
Arguments are mapped from the API types (`[]api.Argument`) into a standard string map. The function must parse `$(VARIABLE)` syntax in values and replace it with values from the context.
Ensure robust unit tests cover empty values, missing context variables, and malformed syntaxes in `interpolate_test.go`.
