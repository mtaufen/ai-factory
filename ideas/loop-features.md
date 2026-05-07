# AI Factory Loop Features

NOTE: Implementation Frameworks
* **Loop Implementation:** Will be built using [adk-go](https://github.com/google/adk-go).
* **Operator:** (Not building yet) Will be implemented using [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime).

## Core Configuration & API Types
* **KRM API Types:** Define Go structs and schemas for `Run`, `Loop`, `Agent`, and `LocalMCPServer` resources.
* **Variable Interpolation:** Implement support for expanding variables (e.g., `$(VAR)`) from `args` within configurations like prompts and tool arguments.

## Loop Execution Engine
* **Loop State Machine:** The core runner that processes a `Loop` definition, starting at `start`, executing steps, and handling basic transitions (`next`, `return`, `retry`).
* **Nested Loops:** Support for a step to invoke another `Loop` as its action, including passing arguments down into the nested loop.
* **Execution Limits:** Implement tracking and enforcement of `maxSteps` per loop and `globalMaxSteps` across an entire `Run`.

## Context & History Management
* **History Forwarding:** Infrastructure to pass execution history from one step to the next based on step rules (`none` or `full`).
* **History Summarization:** Implement the `summary` history forwarding mode, using an LLM to compress the context before passing it to the next step.

## Agent & Tool Integration
* **MCP Client Integration:** Mechanism to connect to local MCP servers over named pipes and route tool execution requests.
* **Direct MCP Tool Steps:** Capability for a loop step to directly invoke an MCP tool without an LLM agent in the middle.
* **Agent Steps:** Capability for a loop step to instantiate and prompt an agent (reading its `agent.md` definition) and passing it specific arguments ("agents as tools" pattern).
* **Agent Tool Filtering:** Implement filtering of available tools based on the allowlist defined in the `Agent` resource.
* **Agent Control Tools:** Injecting internal `pass` and `fail` tools into an agent's context so it can yield control back to the loop state machine with a status message.

## Kubernetes Integration (Operator)
* **Custom Resource Definitions (CRDs):** Manifests to install `Run`, `Loop`, `Agent`, and `LocalMCPServer` types into a Kubernetes cluster.
* **Run Controller:** Operator logic that watches for `Run` resources and creates corresponding Kubernetes Jobs.
* **Pod/Job Scaffolding:** Logic to assemble the Job's pod template, including init containers for network egress/pipe setup, sidecars for MCP servers, volume mounts (emptyDirs, secrets), and the main `factory runtime loop` container.
