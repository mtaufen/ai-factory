---
name: loop-history-forwarding
---
Update the state machine in `factory/pkg/runtime/engine/runner.go` (created in loop-execution-engine) to maintain and forward the `History` context. Implement the transition logic handling `history: none` (clear context), `history: full` (append context), and `history: summary` (invoke the `Summarizer`). Return the final history along with pass/fail and message from `ExecuteLoop`. Add tests verifying the context is correctly forwarded or dropped in `runner_test.go`.
