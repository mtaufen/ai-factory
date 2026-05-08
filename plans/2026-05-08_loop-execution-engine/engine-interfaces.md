---
name: engine-interfaces
---

This task defines the foundational interfaces and types for the Loop Execution Engine.
Create `factory/pkg/runtime/engine/types.go` and define the `Runner` struct and `StepExecutor` interface.

The `Runner` must track the `Run` state, available `Loop`s, and `globalSteps`.
The `StepExecutor` interface should allow the `Runner` to execute steps independently of their implementation type (e.g. MCP, Loop, or Agent actions), returning a boolean `pass`, string `message`, and `error`.
Make sure to include context propagation in the interfaces, as required by the spec.
