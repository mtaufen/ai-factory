---
name: adk-tool-adapters
---
Implement the `tool.Tool` and `toolinternal.FunctionTool` interfaces for our MCP tools in `factory/pkg/runtime/engine/adk_tools.go`. Note the hint in the spec about `google.golang.org/adk/tool/mcptoolset` and `github.com/modelcontextprotocol/go-sdk` - if you can utilize ADK's built-in `mcptoolset.New()`, you can do that instead of manually implementing the tool wrapping. Otherwise, adapt our custom `GetAgentTools` to wrap MCP tools appropriately. Provide unit tests in `factory/pkg/runtime/engine/adk_tools_test.go`.
