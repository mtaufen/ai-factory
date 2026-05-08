---
name: loop-adk-integration
deps:
  - loop-agent-integration
---

# Title

Loop ADK Integration

## Overview

The current Loop Execution Engine implements a custom, simplified LLM loop within the `AgentExecutorImpl` to satisfy the requirements of `loop-agent-integration` and allow mocking during tests. This spec defines how to migrate from that bespoke loop implementation to fully utilizing the `google.golang.org/adk` framework for all Agent executions, while retaining our high-level declarative API (`api.Agent`, `api.Loop`, `api.Step`), and continuing to support our test-driven development workflow via mocking.

## Goals

* Replace the custom `for turns < maxTurns` LLM loop in `agent_executor.go` with an `adk-go` based execution flow.
* Convert our custom MCP/synthetic tools into ADK-compatible tools.
* Wire ADK's `agent.Run` / `runner.Run` into our `StepExecutor` seamlessly.
* Ensure we can still mock the LLM for unit tests without hitting a real backend.
* Preserve the strict `pass` and `fail` transition semantics dictated by our synthetic control tools.

## Non-Goals

* Changing the API surface: The `api.Agent`, `api.Loop`, and `api.Step` types and YAML schemas MUST remain untouched.
* Modifying MCP Client Logic: `factory/pkg/mcp/client.go` will remain as is.

## Key Requirements

1.  **Tool Adaptation**: We must wrap our `mcp.Tool` schema outputs and `CallTool` execution paths into ADK's `tool.Tool` interface (or more specifically `toolinternal.FunctionTool`) so that ADK can provide them to the model and parse the responses natively.
2.  **Synthetic Tools via ADK**: Our custom `pass` and `fail` tools must be rewritten as ADK tools, ensuring that when the agent invokes them, we capture the message and short-circuit the ADK invocation so control yields back to the Loop Execution Engine.
3.  **Mockability**: To satisfy unit testing without a live LLM API key, we must supply ADK with a mocked `model.LLM` implementation, or use a custom tool/callback structure in the tests to simulate the model's trajectory, replacing the current `LLMMockClient` interface.
4.  **History & Context Mapping**: The `currentHistory` maintained by the Loop Engine must be translated into ADK's session history (e.g. `session.Event`) at the start of execution, and new events must be extracted and translated back into `history.History` messages upon conclusion.

## Design

### 1. Tool Wrapping (`adk_tools.go`)
Instead of `GetAgentTools` returning raw `mcp.Tool` objects, we will create an adapter that implements ADK's `tool.Tool` and `toolinternal.FunctionTool` interfaces.
*   **Adapter Struct**: A struct holding the MCP tool schema and a reference to our `mcp.Client`.
*   **`Declaration()`**: Returns a `genai.FunctionDeclaration` constructed from the `mcp.Tool`'s `InputSchema`.
*   **`Run(ctx tool.Context, args any)`**: Unmarshals the args and forwards them to our `CallAgentTool` (or directly to the `mcp.Client`), returning the formatted output.

### 2. Synthetic Control Tools
Instead of explicitly checking `if toolCall == "pass"` in a custom loop, we will create two ADK `FunctionTool`s: `passTool` and `failTool`.
*   These tools will take a `message` string argument.
*   When executed, they will store the message in a side-channel (e.g., a shared state object in the context) and then trigger an immediate cancellation or invocation end (e.g., using `ctx.EndInvocation()`).
*   Alternatively, they can just return the value, but we must use an `AfterToolCallback` or a similar hook in ADK to detect that a control tool was called and immediately halt the agent's turn.

### 3. Agent Execution Migration (`agent_executor.go`)
The `AgentExecutorImpl.Execute` method will be refactored as follows:
*   **Initialize ADK Runner**: We will instantiate an ADK agent using `llmagent.New(cfg)`.
    *   `cfg.Model` will be the LLM model (can be injected as a dependency to support mocking).
    *   `cfg.Tools` will be the slice of wrapped ADK tools.
    *   `cfg.Instruction` will be the loaded prompt from `Agent.spec.path` plus `step.Agent.Prompt`, with args interpolated.
*   **Execute**: We will call `adk_runner.Run(ctx, ...)` or manually invoke `agent.Run(ctx)`.
*   **Evaluate Result**: We will iterate over the returned `iter.Seq2[*session.Event, error]`. We look for the final event or the side-channel state set by the `pass`/`fail` tools to determine the boolean outcome and the message.
*   **History**: We map the generated `session.Event` objects back to `history.Message` objects (Role "model" or "tool").

### 4. Mocking ADK (`agent_executor_test.go`)
Since ADK is designed to be model-agnostic, we can create a struct that implements the `model.LLM` interface defined in `google.golang.org/adk/model`.
*   The mocked `GenerateContent` method will return a predefined sequence of `genai.FunctionCall` responses (to simulate the LLM requesting a tool) and eventually a call to `pass` or `fail`.
*   This replaces our bespoke `LLMMockClient` with ADK's native dependency injection model.

## Tests

* Update `agent_executor_test.go` to use a mock implementing `model.LLM`.
* Verify that the custom MCP adapter tools correctly translate schemas to `genai.FunctionDeclaration`.
* Verify that the ADK agent correctly yields execution and propagates the message when the `pass` or `fail` tool is executed.
