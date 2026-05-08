---
name: loop-agent-execution
---

Implement the execution of `agent` steps within the loop runner. This involves loading the `Agent` resource, reading the prompt from `Agent.spec.path`, appending any inline prompt, and interpolating step `args`. You should inject the current history (from `loop-history-management`) and the allowed tools. Execute the agent's LLM loop until it either calls the `pass` or `fail` tool, or hits its internal interaction limit (which should result in a failure). Use a mocked LLM client to write unit tests for this step execution logic.
