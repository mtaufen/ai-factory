---
name: test-loop-pod
---

Implement the `TestLoopPodIntegration` test in `integration_test.go`.
This test must create an `api.Loop` and `api.Agent` configuration that uses tools from multiple `api.LocalMCPServer` definitions (e.g. `dev-mcp` and `git-mcp`). Use pipes (`io.Pipe` or similar) to wire the test MCP manager to the background mock MCP server goroutines. Verify that the agent successfully executes a task by making tool calls across the different MCP servers and returning a final result. Ensure no real network or host commands are executed.
