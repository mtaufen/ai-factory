---
name: cmd-run-loop
---

Complete the `factory runtime loop` implementation in `factory/cmd/factory/runtime/loop/loop.go`.
1. Initialize `mcp.NewConnectionManager()`.
2. Initialize the real Gemini model using `gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{})` (ensure the `GEMINI_API_KEY` is passed or picked up automatically).
3. Use the `adk_adapter` to wrap the Gemini model and pass it to `history.NewLLMSummarizer()`.
4. Initialize the executors: `engine.NewAgentExecutor`, `engine.NewMCPStepExecutor`, and a no-op executor for the standard runner step.
5. Create a `Runner` with these executors and the parsed structured maps, and call `Runner.ExecuteLoop()` on the start loop.
