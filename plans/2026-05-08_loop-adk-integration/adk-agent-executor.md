---
name: adk-agent-executor
---
Refactor `AgentExecutorImpl.Execute` in `factory/pkg/runtime/engine/agent_executor.go` to use `adk-go` natively via `llmagent.New(cfg)` and `agent.Run(ctx)`. You will need to implement the synthetic `passTool` and `failTool` directly as ADK tools that halt the loop invocation and store their message. Critically, you must map the engine's `currentHistory` to ADK session history before execution, and extract `session.Event` objects mapping them back to `history.Message` objects after execution. For tests, update `factory/pkg/runtime/engine/agent_executor_test.go` to use a mocked `model.LLM` interface that yields `genai.FunctionCall` responses to simulate the interaction loop.
