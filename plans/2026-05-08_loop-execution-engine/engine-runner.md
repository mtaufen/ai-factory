---
name: engine-runner
---

This task implements the core state machine runner for the Loop Execution Engine.
Create `factory/pkg/runtime/engine/runner.go` to implement `ExecuteLoop(ctx context.Context, loopName string, args map[string]string) (bool, string, error)`.

Requirements:
* Implement step transitions based on success/failure outcomes mapped to `pass` and `fail` configuration (`next: <step>`, `retry`, `return`).
* It must strictly separate Go system errors (fatal) from step failures (`pass = false`).
* Limits tracking: Strictly enforce `globalMaxSteps` across the run, and `maxSteps` per local loop.
* Context propagation and cancellation: The `ctx context.Context` must be threaded through all step executions. If the context is cancelled at any point, the runner must abort immediately and return the context error.
* Nested Loops and Cyclic Dependency Prevention: When a nested `LoopAction` is encountered, it should be interpolated and execute the nested loop. To prevent call stack overflows during deep recursion, implement a `maxDepth` limit or use an execution strategy (like a stack-based iterative runner) that is guaranteed not to overflow.
* Write thorough unit tests in `runner_test.go` using stubbed `StepExecutor` implementations to verify transitions, limits, context cancellation, and recursion depth/cycle prevention without relying on real tool execution.
