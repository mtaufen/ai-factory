---
name: adk-model-history-adapter
---

Create an adapter in `factory/pkg/runtime/history/adk_adapter.go` that wraps a `google.golang.org/adk/model.LLM`.
The adapter must implement `history.LLMClient` (from `factory/pkg/runtime/history`), which has the signature:
`Generate(ctx context.Context, prompt string) (string, error)`

The `model.LLM` interface uses `adk/model.GenerateRequest` and returns a `*adk/model.GenerateResponse`. Map the prompt into the request and extract the text from the response. This adapter will be used to initialize the summarizer.
