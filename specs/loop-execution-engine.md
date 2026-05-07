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

*   **`ExecuteLoop(loopName string, args map[string]string) error`**: Starts execution at the loop's `spec.start` step.
*   **Transitions**: After a step executes, it returns a boolean (pass/fail) and a message. The runner uses the step's `pass` or `fail` configuration to determine the next step:
    *   If `next` is a step name: Transition to that step.
    *   If `next` is `retry`: Re-run the current step.
    *   If `next` is `return`: Exit the current `ExecuteLoop` call and return control to the parent (or finish the `Run` if it's the root loop).
*   **Variable Scope**: Steps can pass args down. The args map should be interpolated against the current scope before being passed.

### Execution Limits
The `Runner` must maintain two counters:
*   `globalSteps`: Tracks every single step executed across all loops. Fails the Run if it exceeds `Run.spec.globalMaxSteps`.
*   Local `loopSteps`: An integer passed down or maintained in the call stack. Fails the specific loop if it exceeds `Loop.spec.maxSteps`.

### Nested Loops
When a step has a `loop` action, `ExecuteLoop` is called recursively with the nested loop's name and interpolated arguments.

## Tests

* Unit tests for the state machine logic using a stubbed step executor that returns predefined pass/fail results.
* Tests for `next: <step>`, `next: return`, and `next: retry`.
* Tests verifying `globalMaxSteps` and `maxSteps` properly abort execution when exceeded.
* Tests verifying arguments are properly interpolated and passed to nested loops.
