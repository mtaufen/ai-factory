---
name: loop-execution-engine
deps:
  - loop-api-types
---

# Title

Loop Execution Engine

## Overview

The Loop Execution Engine is the core state machine responsible for processing a `Loop` definition. It handles step execution, control flow (`next`, `return`, `retry`), nested loops, and enforces execution limits.

## Goals

* Implement the main state machine runner that takes a `Run` and entrypoint `Loop`.
* Implement step transitions based on success/failure outcomes mapped to `pass` and `fail` configuration.
* Support nested loops by allowing a step's action to invoke another `Loop`.
* Track and enforce `maxSteps` (per loop) and `globalMaxSteps` (per Run).

## Non-Goals

* Implementing actual MCP clients or agent generation logic (these will be stubbed or built in subsequent specs).
* Complex history management (history forwarding and summarization will be handled in another spec).

## Design

### State Machine Runner
Create a `Runner` struct instantiated with the loaded `Run` and a registry of available `Loop`s.
*   **`ExecuteLoop(loopName string, args map[string]string) (bool, string, error)`**: Starts execution at the loop's `spec.start` step, returning pass/fail status, the final message, and any execution errors.
*   **Transitions**: After a step executes, it evaluates to a boolean (pass/fail) and a message. The runner uses the step's `pass` or `fail` configuration to determine the next step:
    *   If `next` is a step name: Transition to that step.
    *   If `next` is `retry`: Re-run the current step.
    *   If `next` is `return`: Exit the current `ExecuteLoop` call and return the current pass/fail status and message to the parent loop (or finish the `Run` if it's the root loop).
*   **Variable Scope**: Steps can pass args down. The args map should be interpolated against the current scope before being passed.

### Execution Limits
The `Runner` must maintain two counters:
*   `globalSteps`: Tracks every single step executed across all loops. Fails the Run if it exceeds `Run.spec.globalMaxSteps`.
*   Local `loopSteps`: An integer passed down or maintained in the call stack. Fails the specific loop if it exceeds `Loop.spec.maxSteps`.

### Nested Loops
When a step has a `loop` action, `ExecuteLoop` is called recursively with the nested loop's name and interpolated arguments.



## Examples

```yaml
kind: Loop
metadata:
  name: spec-review-main
spec:
  start: clone-step
  steps:
  - name: clone-step
    mcp: # calls an MCP tool directly
      name: git-mcp
      tool: clone_repository
      args:
      - name: remote
        value: "$(REPO_URL)"
    pass:
      next: spec-review-step
    fail:
      next: return # return is a reserved keyword that returns message and history to the parent
  - name: spec-review-step
    loop: # runs a nested loop
      name: spec-review-loop # name of the loop
      args: # passed to the nested loop
      - name: IDEA
        value: "$(IDEA)"
    pass:
      next: push
    fail:
      next: return
  - name: push
    mcp:
      name: git-mcp
      tool: push_branch
      args:
      - name: remote
        value: "$(REPO_URL)"
      - name: branch
        value: main
    pass:
      next: return # last step, so return to the parent on pass
    fail:
      next: return
---
kind: Loop
metadata:
  name: spec-review-loop
spec:
  start: speccer-step
  maxSteps: 25 # limit on maximum number of step executions. Each step execution counts as one, even if the step is a loop. That sub-loop would have its own internal maxSteps limit.
  steps:
  - name: speccer-step
    agent:
      name: speccer-agent
      prompt: "Generate a spec for this idea (check args for idea)."
      args:
      - name: idea
        value: "$(IDEA)"
    pass:
      next: spec-format-step
    fail:
      next: retry # retry is a keyword that just keeps repeating the step until it succeeds
  - name: spec-format-step
    agent:
      name: spec-format-agent
      prompt: "Validate the format of new spec files."
    pass:
      next: spec-review-step
    fail:
      next: speccer-step # loop back for rework on failure
  - name: spec-review-step
    agent:
      name: spec-review-agent
      prompt: "Review the spec for consistency with the idea, and consistency with adjacent specs."
      args:
      - name: idea
        value: "$(IDEA)"
    pass:
      next: return
    fail:
      next: speccer-step
```

## Tests

* Unit tests for the state machine logic using a stubbed step executor that returns predefined pass/fail results.
* Tests for `next: <step>`, `next: return`, and `next: retry`.
* Tests verifying `globalMaxSteps` and `maxSteps` properly abort execution when exceeded.
* Tests verifying arguments are properly interpolated and passed to nested loops.

