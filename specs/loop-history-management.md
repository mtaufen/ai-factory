---
name: loop-history-management
deps:
  - loop-execution-engine
---

# Title

Loop History Management

## Overview

Implements context and history forwarding between steps in the loop execution engine. This ensures context is either passed, dropped, or summarized according to the step's `history` rules.

## Goals

* Implement infrastructure to pass execution history (messages, agent turns, tool calls) from one step to the next.
* Implement the three history forwarding modes: `none`, `full`, and `summary`.
* Use an LLM to compress context when the `summary` mode is used.

## Non-Goals

* Implementing the actual Agent or Tool execution logic. We are only focused on how history context strings or message arrays are passed between them.

## Design

### History Types
Define a generic `History` struct or type alias (e.g., a slice of `Message` structs or just structured strings) to represent the context of execution up to that point.

### Forwarding Logic
Within the `Runner`'s transition logic (from `loop-execution-engine`), modify it to carry a `currentHistory` object.
When transitioning from `Step A` to `Step B`:
1.  Check the `history` field in `Step A`'s outcome (`pass` or `fail`).
2.  If `history: none` (default), clear the `currentHistory` before starting `Step B`.
3.  If `history: full`, append the new messages from `Step A` to `currentHistory` and pass it to `Step B`.
4.  If `history: summary`, invoke an LLM summarization service that takes the `currentHistory` and `Step A`'s new messages, generates a concise text summary, and creates a new `currentHistory` containing only the summary.

### Summarization Service
Create a `Summarizer` interface. Provide a simple LLM-based implementation that prompts the model to summarize the previous context and the latest result for the next step.

## Tests

* Unit tests verifying `history: none` correctly drops all previous context.
* Unit tests verifying `history: full` concatenates context across multiple steps.
* Unit tests with a mocked `Summarizer` to verify `history: summary` calls the summarizer and forwards the result.
