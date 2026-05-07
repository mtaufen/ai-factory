---
name: loop-mcp-integration
deps:
  - loop-api-types
  - loop-execution-engine
---

# Title

Loop MCP Integration

## Overview

Connects the Loop Execution Engine to local Model Context Protocol (MCP) servers and enables loops to execute MCP tools directly.

## Goals

* Implement an MCP client capable of connecting to local MCP servers over named pipes.
* Add capability to the Loop Execution Engine to execute `mcp` steps directly.

## Non-Goals

* We are not building the actual MCP servers (e.g., git-mcp). We are only building the client and the step execution logic.
* LLM Agent tool usage is not in this spec (that belongs in `loop-agent-integration`).

## Design

### MCP Client Connection
*   Read `LocalMCPServer` resources to discover the `pipe` path for each server.
*   Implement a connection manager that establishes communication over the specified named pipes using standard MCP JSON-RPC protocol.

### Direct MCP Tool Steps
*   Extend the `Runner` to handle steps where the action is `mcp`.
*   When executing an `mcp` step:
    1.  Locate the MCP client corresponding to `mcp.name`.
    2.  Interpolate `mcp.args` using the current scope's variables.
    3.  Call the tool `mcp.tool` with the interpolated arguments.
    4.  Evaluate the tool's response to determine if the step passes or fails (e.g., based on tool error return).
    5.  Include the tool's output in the step's resulting message.



## Examples

```yaml
kind: LocalMCPServer # identifies where to find a local MCP server; the server itself is configured externally
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: git-mcp
spec:
  pipe: /var/run/mcp/git-mcp
---
kind: Loop
metadata:
  name: mcp-demo-loop
spec:
  start: clone-step
  steps:
  - name: clone-step
    mcp: # calls an MCP tool directly
      name: git-mcp # name of the mcp server
      tool: clone_repository # name of the tool
      args: # mcp tool arguments
      - name: remote # argument name
        value: "$(REPO_URL)" # value interpolated from args, in this case configured by Run
    pass:
      next: return
    fail:
      next: return
```

## Tests

* Unit tests for the named pipe connection logic using a dummy pipe and mocked MCP server responses.
* Integration of the `mcp` step handler in the `Runner`, testing with a mocked MCP client to verify tool calls, argument interpolation, and pass/fail evaluation.

