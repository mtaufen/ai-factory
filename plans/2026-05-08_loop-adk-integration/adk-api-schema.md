---
name: adk-api-schema
---
Update `factory/pkg/runtime/api/types.go` with the API Schema Evolution described in the `loop-adk-integration` spec. Deprecate the top-level `Path` in `AgentSpec` and add `Prompt` (`*PromptSource`), `SubAgents` (`[]SubAgent`), `Temperature` (`*float32`), and `MaxTokens` (`*int32`). Ensure you define the `PromptSource`, `ConfigMapRef`, and `SubAgent` structs as specified. Ensure that using `ConfigMapRef` results in an error during validation when running locally.
