---
name: loop-api-types-vars
---

Implement the variable interpolation utility `ExpandVariables(input string, args map[string]string) string` in the `api` package.
This function should replace instances of `$(VAR)` in strings using the provided map of arguments.
If a variable is not defined, it must evaluate to an empty string. Ensure the implementation handles missing or malformed variables gracefully without panicking.
Provide comprehensive unit tests in `vars_test.go` covering combinations of defined, undefined, and malformed variables.
