---
name: mcp-step-executor
---

# MCP Step Executor

Integrate the MCP client with the Loop Execution Engine by extending the `Runner` to handle `mcp` steps.

**Key Requirements:**
1. When executing an `mcp` step, the `Runner` (or a dedicated MCP step handler in `mcp.go`) must locate the MCP client corresponding to `mcp.name` (this relies on resolving the `LocalMCPServer` resource to its `pipe` path).
2. The step executor must interpolate `mcp.args` using the current scope's variables.
3. Call the tool specified by `mcp.tool` with the interpolated arguments using the MCP client.
4. Evaluate the tool's response:
   - If the MCP tool returns a JSON-RPC error payload, this should trigger a step failure (evaluates to `pass = false`) and should *not* crash the Runner or return a fatal system error.
   - On success, it evaluates to `pass = true`.
5. Include the tool's output or error details in the step's resulting message.
6. Provide tests in `mcp_test.go` with a mocked MCP client to verify tool calls, argument interpolation, and pass/fail evaluation based on tool responses. Ensure `runner.go` integrates the MCP step handling logic cleanly.
