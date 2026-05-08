---
name: mcp-client
---

# MCP Client

Implement an MCP client in `factory/pkg/mcp/client.go` capable of connecting to local MCP servers over OS-level named pipes (FIFOs).

**Key Requirements:**
1. Connection MUST be over actual OS-level named pipes, reading and writing directly to the pipe file. DO NOT use Unix Domain Sockets.
2. The connection manager should maintain a long-lived, thread-safe connection to the pipe for the duration of the process.
3. Use the standard MCP JSON-RPC protocol over the pipe.
4. Implement connection logic that gracefully handles JSON-RPC error payloads (they should not crash the connection but be returned as results).
5. Provide unit tests using a dummy pipe and mocked MCP server responses in `client_test.go`.
