---
name: loop-agent-integration
deps:
  - loop-mcp-integration
  - loop-history-management
---

# Title

Loop Agent Integration

## Overview

Enables the Loop Execution Engine to invoke LLM agents as steps, including injecting MCP tools based on an allowlist and providing built-in control tools for the agent to yield execution back to the loop.

## Goals

* Implement execution of `agent` steps within the loop runner.
* Filter available MCP tools based on the allowlist defined in the `Agent` resource.
* Inject `pass` and `fail` control tools into the agent's context.

## Non-Goals

* Implementing the internal logic of the `adk-go` framework. We will utilize it as a library.

## Design

### Agent Step Execution
*   When the `Runner` encounters a step where the action is `agent`:
    1.  Load the `Agent` resource definition.
    2.  Read the agent's prompt from the file specified in `Agent.spec.path` (`agent.md`).
    3.  Append any inline `prompt` from the step definition.
    4.  Interpolate step `args` and provide them to the agent (the "agents as tools" pattern).
    5.  Pass the `currentHistory` (from `loop-history-management`) to the agent so it has context.

### Tool Filtering and Injection
*   **Tool Filtering**: Iterate over the `Agent.spec.tools` allowlist. Request the schema for each allowed tool from the corresponding MCP client (from `loop-mcp-integration`). Do not provide tools to the agent that are not in the allowlist.
*   **Control Tools**: Implement two synthetic tools and inject them into the agent's context:
    *   `pass(message string)`: Indicates the agent successfully completed its task.
    *   `fail(message string)`: Indicates the agent encountered an unrecoverable error or could not make progress.

### Agent Loop Termination
*   The agent's internal LLM loop runs until it calls either the `pass` or `fail` synthetic tool.
*   The argument provided to `pass` or `fail` becomes the step's outcome message, and the step transitions accordingly.
*   If the agent exceeds its internal interaction limits without calling `pass` or `fail`, the step should terminate as a `fail`.

## Tests

* Unit tests using a mocked LLM client to simulate an agent interacting with tools and eventually calling `pass` or `fail`.
* Tests verifying that only allowed tools are presented to the agent.
* Tests verifying that inline prompts, agent definitions, arguments, and history are correctly assembled and passed to the agent.
