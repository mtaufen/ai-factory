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

## Examples

```yaml
kind: Run
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: spec-developer-12345
spec:
  globalMaxSteps: 1000 # global limit on max steps, accounted recursively for all loops and sub-loops. This is different from Loop maxSteps which is only 1 level deep, for that specific loop.
  start: spec-review-main # name of the entrypoint Loop
  args: # args passed to start. Semantics same as K8s env vars and supports downward API semantics too.
  - name: IDEA
    value: "I want to build an agent to do X"
  - name: REPO_URL
    value: "https://www.github.com/user/repo"
# convention: passing runs exit with code 0, failing runs exit with nonzero code
---
kind: Loop
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: spec-review-main
spec:
  start: clone-step
  steps:
  - name: clone-step
    mcp: # calls an MCP tool directly
      name: git-mcp # name of the mcp server
      tool: clone_repository # name of the tool
      args: # mcp tool arguments
      - name: remote # argument name
        value: "$(REPO_URL)" # value interpolated from args, in this case configured by Run
    pass: # what to do on pass
      next: spec-review-step # step to move on to on pass
      message: "cloned $(REPO_URL) successfully" # message passed to the next step
      history: none # whether to forward context, default is "none" to start next step with fresh context
    fail: # what to do on fail
      next: return # return is a reserved keyword that returns message and history to the parent
      message: "failed to clone $(REPO_URL)" # failure message to pass to next step
      history: full # passes the full history from this step to the next step
---
kind: Agent
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: speccer-agent
spec:
  path: /etc/agents/speccer/agent.md # path to agent.md definition
  tools:
  - mcp: 
      name: dev-mcp # name of MCP server
      tools: # allowlist of tools
      - name: ReadFile
      - name: WriteFile
---
kind: LocalMCPServer # identifies where to find a local MCP server; the server itself is configured externally
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: git-mcp
spec:
  pipe: /var/run/mcp/git-mcp
```

## Tests

* Unit tests for YAML unmarshaling of each type, using snippets from `ideas/loop.md` as test cases.
* Unit tests for `ExpandVariables` with various combinations of defined, undefined, and malformed variables.

