---
name: loop-agent-execution
---

Implement the execution of `agent` steps within the loop runner.

### 1. AgentExecutor Implementation (`agent_executor.go`)
Create an `AgentExecutor` implementing the `StepExecutor` interface. This executor must:
- Load the `Agent` resource definition.
- Read the prompt from `Agent.spec.path` and append any inline `prompt` from the step.
- Interpolate step `args`.
- Inject the current history and filtered MCP tools (from `loop-agent-control-tools`).
- Execute the agent's LLM loop using the `adk-go` framework (note: `adk-go` must be added as a dependency in `go.mod`). The loop runs until the agent calls synthetic `pass` or `fail` tools, or exceeds its interaction limit.

### 2. Engine Wiring
- **`engine/types.go`**: Add `AgentExecutor StepExecutor` to the `Runner` struct.
- **`engine/runner.go`**: Update `ExecuteLoop` to explicitly dispatch to `AgentExecutor` when `step.Agent != nil`.

### 3. Testing (`agent_executor_test.go`)
Write unit tests using a mocked LLM client to verify correct prompt assembly, tool injection, history passing, and loop termination controls.
