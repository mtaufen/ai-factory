---
name: mock-mcp-servers
---

Implement the mock MCP servers and LLM client needed for the integration test.
Create `mock_mcp_test.go` and add the "dev" mock MCP server that simulates the Pod approach. It should provide `ReadFile`, `WriteFile`, and `RunCommand` tools. The file tools should operate on a configurable temporary directory, and `RunCommand` should just return a successful dummy string. Also, define a mock for the `model.LLM` interface that can yield a deterministic sequence of tool calls and responses, simulating the agent's behavior.
