---
name: loop-api-types
deps: []
---

# Title

Loop API Types

## Overview

Defines the core Kubernetes Resource Model (KRM) API types and variable interpolation support for the AI Factory Loop Execution Engine. These types specify the configuration for runs, loops, agents, and MCP servers.

## Goals

* Define Go structs with YAML struct tags for `Run`, `Loop`, `Agent`, and `LocalMCPServer` mirroring KRM APIs.
* Provide a mechanism to parse and load these configurations.
* Implement a variable interpolation system (e.g., `$(VAR)`) to substitute values from `args` into strings.

## Non-Goals

* We are NOT implementing Kubernetes Custom Resource Definitions (CRDs) or a Kubernetes Operator at this time. Only the Go types are needed.
* We are not implementing the actual execution logic in this spec.

## Design

### Go Structs
Create Go structs for the four main resources using `factory.ai.gke.io/v1alpha1` style schema.

*   **`Run`**: Represents an execution instance. Should have `spec.globalMaxSteps` (int), `spec.start` (string), and `spec.args` (list of name/value pairs).
*   **`Loop`**: A composable control flow primitive. Should have `spec.start` (string), `spec.maxSteps` (int), and `spec.steps` (list of Step).
    *   `Step`: Has `name`, `pass` (NextAction), `fail` (NextAction), and an action which can be one of: `mcp`, `loop`, or `agent`.
    *   `NextAction`: Has `next` (string keyword like `return`, `retry`, or a step name), `message` (string), and `history` (enum: `none`, `full`, `summary`).
*   **`Agent`**: Defines an LLM agent. Has `spec.path` (string to agent.md) and `spec.tools` (allowlist of MCP servers and tools).
*   **`LocalMCPServer`**: Configuration for a local MCP server. Has `spec.pipe` (string pointing to a named pipe).

### Variable Interpolation
Create an interpolation utility `ExpandVariables(input string, args map[string]string) string`. It should replace instances of `$(VAR)` in strings using the provided map of arguments. If a variable is not defined, it should evaluate to an empty string.

## Tests

* Unit tests for YAML unmarshaling of each type, using snippets from `ideas/loop.md` as test cases.
* Unit tests for `ExpandVariables` with various combinations of defined, undefined, and malformed variables.
