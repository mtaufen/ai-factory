---
name: engine-runner-refactor
---

# Engine Runner Refactor

This task updates the Loop Execution Engine's main `Runner` to use an explicit `StackFrame` model instead of Go recursion. 
You will be modifying the `runner.go` file. You need to implement the `StackFrame` struct and refactor `ExecuteLoop` into a flat loop that maintains a slice of `StackFrame`s. 
When a nested loop is invoked, push the current frame. When a `return` action is processed, pop the current frame and resume the parent. Make sure to retain all the current state checks: `globalMaxSteps`, `maxSteps`, and context cancellation. Run the tests in `runner_test.go` and update them if necessary (though the public API should remain mostly unchanged, tests relying on the exact previous recursive behavior or depth checks might need adjustments).
