---
name: loop-integration-test
deps:
  - loop-execution-engine
  - loop-mcp-integration
  - loop-adk-integration
---

# Loop Pod Integration Test

## Overview

We need to validate that `factory runtime loop` can coordinate an agent execution utilizing tools provided by multiple local MCP servers communicating over stdio pipes. Specifically, we want an integration test that runs as a standard `go test` and simulates the target Pod-like architecture where different MCP servers provide different sets of tools, including a "dev" MCP server that restricts the agent to a mock filesystem and shell.

## Goals

- Create a `go test` integration test that verifies loop execution with multiple MCP servers.
- Simulate the "Pod approach" architecture described in `ideas/loop.md` where the `loop` binary coordinates with sidecar MCP servers.
- Implement a mock "dev" MCP server for the test that provides mock filesystem and shell capabilities without relying on a real network or risking host execution.
- Validate that the Loop engine passes the correct arguments and receives valid tool responses over named pipes (or simulated stdio connections) using the MCP protocol.
- Prove that the loop can resolve a complex task by using tools from multiple different MCP servers.

## Non-Goals

- Do not test actual Kubernetes deployments, Pods, or Jobs yet.
- Do not build production-ready sidecar MCP server images.
- Do not make external network requests or LLM API calls during the test (use a mocked LLM).

## Key Requirements

- **No Real Commands**: The dev MCP server must execute tools against an entirely in-memory mock or a temporary test directory, not the user's host filesystem. Shell commands should be mocked or restricted to a sandboxed set of dummy commands.
- **Go Test Idiomatic**: The test should use standard `testing` package patterns. Wait groups or similar synchronization may be needed to manage the lifecycles of the mocked MCP servers alongside the engine execution.
- **Mock LLM Client**: Rely on `mockADKModel` or a similar mock of the `model.LLM` interface to provide deterministic GenAI responses that simulate the agent invoking the MCP tools.

## Design

### Test Setup

1. **MCP Server Simulation**: Instead of full sidecar containers, the test should spawn background goroutines that run minimal MCP server implementations.
2. **Pipes/Stdio**: The mock MCP servers should communicate with the `mcp.ConnectionManager` using simulated stdio pipes (e.g., via `io.Pipe` or `os.Pipe`) to closely mirror how the `factory runtime loop` will communicate with local KRM `LocalMCPServer` resources.
3. **Mock Dev MCP**:
    - Build a minimal MCP server within the test suite that advertises tools like `ReadFile`, `WriteFile`, and `RunCommand`.
    - `ReadFile` and `WriteFile` should operate on a temporary directory created via `t.TempDir()`.
    - `RunCommand` should just record that a command was requested and return a mock successful stdout string.

### Test Execution Flow

1. Define a KRM `api.Loop` and an `api.Agent` that specifies a task requiring tools from the "dev" MCP server and another dummy MCP server (e.g. "git-mcp").
2. Define the `api.LocalMCPServer` configurations pointing to the pipes established in the setup phase.
3. Use a mocked LLM that deterministically yields `genai.FunctionCall` responses corresponding to the advertised tools on the respective MCP servers.
4. Execute the loop via `AgentExecutorImpl.Execute()`.
5. The executor will start the runner, which talks to the LLM mock, receives function calls, dispatches them to the MCP manager, and retrieves results.

## Tests

- `TestLoopPodIntegration`: The core integration test verifying that an agent can complete a multi-step task by executing tools sourced from multiple independent MCP servers over pipes.
  - Verifies that the loop engine successfully resolves tool metadata from both servers.
  - Verifies that tool executions correctly route to the specific MCP server that provides the tool.
  - Verifies that the final output matches the mocked LLM's final state and that all expected mock tools were called.
