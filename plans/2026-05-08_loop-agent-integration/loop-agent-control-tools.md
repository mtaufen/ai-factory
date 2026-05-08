---
name: loop-agent-control-tools
---

Implement the tool filtering logic based on the `Agent.spec.tools` allowlist. This should iterate over the allowlist, request schemas from the corresponding MCP client, and filter out any unlisted tools. Additionally, implement the `pass(message string)` and `fail(message string)` synthetic control tools. Ensure unit tests verify that unlisted tools are hidden and that the synthetic tools function as expected.
