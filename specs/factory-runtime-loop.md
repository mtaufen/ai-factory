---
name: factory-runtime-loop
deps:
  - loop-api-types
  - loop-execution-engine
  - loop-history-management
  - loop-mcp-integration
  - loop-adk-integration
---

# Factory Runtime Loop: Real LLM

Connect Runtime Engine to Real LLM

## Overview

We will introduce a new CLI subcommand `factory runtime loop` that parses KRM manifests, initializes long-lived MCP connections, initializes a real Gemini LLM client, and executes the loop engine.

## Goals

* Implement the `factory runtime loop` CLI command.
* Connect the engine to real named pipes for MCP servers.
* Connect the engine to the real Gemini API using an API key from the environment.
* Support executing a full `Run` resource from input manifests.

## Non-Goals

* Building the Kubernetes operator/controller (that will be done separately).
* Supporting remote MCP servers over network (only local named pipes for now).

## Key Requirements

* **API Key Handling**: The command MUST retrieve the Gemini API key from the environment (e.g., `GEMINI_API_KEY`) and propagate it securely to the model initialization.
* **Manifest Parsing**: The command MUST read the input file or directory specified by `--config`, parse all unstructured objects, and convert them to their corresponding structured API types.
* **Error Handling**: Any failure during initialization (e.g., missing manifests, failure to connect to named pipes, invalid API key) MUST cause the command to exit with a non-zero status code and an actionable error message.

## Design

### Command Line Interface
Implement the `factory runtime loop` command in `factory/cmd/factory/runtime/loop/loop.go`.
* Flags: `--config <path>` (required) pointing to the YAML manifests defining the `Run`, `Loop`, `Agent`, and `LocalMCPServer` resources.
* Flow:
  1. Read and parse manifests using `api.Parse`.
  2. Convert unstructured objects into structured maps (`map[string]*api.Agent`, etc.) using `runtime.DefaultUnstructuredConverter`.
  3. Locate the `api.Run` resource to determine the entrypoint (`Start` loop).
  4. Initialize `mcp.NewConnectionManager()`.
  5. Initialize real Gemini model via `gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{})`.
  6. Adapt the ADK model to implement `history.LLMClient` for the summarizer.
  7. Initialize `engine.NewAgentExecutor`, `engine.NewMCPStepExecutor`, and a no-op standard executor.
  8. Run the loop engine via `Runner.ExecuteLoop`.

Register the new subcommand in `factory/cmd/factory/runtime/runtime.go`.

## Examples

```bash
GEMINI_API_KEY="AIzaSy..." factory runtime loop --config ./testdata/sample-run.yaml
```

## Tests

* Unit tests for the command setup logic verifying flag parsing and manifest loading.
* Update existing mock MCP servers in `mock_mcp_test.go` to use `github.com/spf13/afero` to mock the filesystem entirely in memory, replacing the usage of standard OS temporary directories.
* Live integration test in `factory/cmd/factory/runtime/loop/loop_live_test.go`:
  - Attempts to load environment variables from the gitignored `.env/.env` file at the project root, populating `GEMINI_API_KEY` and `FACTORY_LIVE_TEST`.
  - Skips the test unless both `FACTORY_LIVE_TEST=true` and `GEMINI_API_KEY` are set in the environment.
  - Reuses the updated afero-backed mock MCP servers and pipe generation from `mock_mcp_test.go`.
  - Defines multiple integration test cases backed by static YAML manifests placed in a `testdata/` directory (e.g., `testdata/simple-run.yaml`).
  - The test reads the consistent manifests from `testdata/`, points `LocalMCPServer` definitions at ephemeral pipes, initializes the real Gemini LLM client, executes the loop, and verifies expected side effects against the afero filesystem.