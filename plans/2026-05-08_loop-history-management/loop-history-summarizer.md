---
name: loop-history-summarizer
---
Implement a concrete `Summarizer` in `factory/pkg/runtime/history/summarizer.go` that uses an LLM. It should take the `currentHistory` and the latest execution results, and prompt the model to generate a concise summary to be passed into the next step's context. Add tests in `summarizer_test.go` using a mock LLM.
