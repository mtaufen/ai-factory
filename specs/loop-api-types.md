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

## Key Requirements

*   **Kubernetes Resource Model (KRM) Compliance:** The Go structs MUST strictly implement standard Kubernetes `TypeMeta` and `ObjectMeta` fields, and include `deepcopy-gen` annotations to ensure compatibility with standard Kubernetes machinery down the line.
*   **YAML Compatibility:** Parsing logic MUST utilize Kubernetes-native `YAMLOrJSONDecoder` to parse multi-document YAML streams into `unstructured.Unstructured` objects, allowing dynamic handling of generic KRM resources.
*   **Variable Interpolation Safety:** The `ExpandVariables` logic MUST cleanly handle undefined variables (e.g. by evaluating to an empty string) and MUST NOT panic or crash the parser if syntax is malformed.

## Design

### Go Structs
Create Go structs for the four main resources using `factory.ai.gke.io/v1alpha1` style schema.
*   **DeepCopy methods**: The types should include `//+k8s:deepcopy-gen=true` and `//+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object` annotations so that standard `controller-gen` tooling can generate `DeepCopy` methods.
*   **YAML Parsing**: Specify that the loader must use Kubernetes-native decoders (e.g., `k8s.io/apimachinery/pkg/util/yaml.NewYAMLOrJSONDecoder`) to support reading multi-document YAML files into `unstructured.Unstructured` objects (from `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured`).

```go
package api

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Run struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RunSpec `json:"spec"`
}

type RunSpec struct {
	GlobalMaxSteps int        `json:"globalMaxSteps"`
	Start          string     `json:"start"`
	Args           []Argument `json:"args,omitempty"`
}

type Argument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Loop struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LoopSpec `json:"spec"`
}

type LoopSpec struct {
	Start    string `json:"start"`
	MaxSteps int    `json:"maxSteps,omitempty"`
	Steps    []Step `json:"steps"`
}

type Step struct {
	Name  string       `json:"name"`
	MCP   *MCPAction   `json:"mcp,omitempty"`
	Loop  *LoopAction  `json:"loop,omitempty"`
	Agent *AgentAction `json:"agent,omitempty"`
	Pass  NextAction   `json:"pass"`
	Fail  NextAction   `json:"fail"`
}

type MCPAction struct {
	Name string     `json:"name"`
	Tool string     `json:"tool"`
	Args []Argument `json:"args,omitempty"`
}

type LoopAction struct {
	Name string     `json:"name"`
	Args []Argument `json:"args,omitempty"`
}

type AgentAction struct {
	Name   string     `json:"name"`
	Prompt string     `json:"prompt,omitempty"`
	Args   []Argument `json:"args,omitempty"`
}

type HistoryMode string

const (
	HistoryNone    HistoryMode = "none"
	HistoryFull    HistoryMode = "full"
	HistorySummary HistoryMode = "summary"
)

type NextAction struct {
	Next    string      `json:"next"`
	Message string      `json:"message,omitempty"`
	History HistoryMode `json:"history,omitempty"`
}

type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AgentSpec `json:"spec"`
}

type AgentSpec struct {
	Path  string         `json:"path"`
	Tools []ToolProvider `json:"tools,omitempty"`
}

type ToolProvider struct {
	MCP *MCPToolSet `json:"mcp,omitempty"`
}

type MCPToolSet struct {
	Name  string       `json:"name"`
	Tools []ToolConfig `json:"tools"`
}

type ToolConfig struct {
	Name string `json:"name"`
}

type LocalMCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LocalMCPServerSpec `json:"spec"`
}

type LocalMCPServerSpec struct {
	Pipe string `json:"pipe"`
}
```

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

