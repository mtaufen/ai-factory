---
name: factory-runtime-loop
deps: []
---

# Factory Runtime Loop

## Overview

The `factory runtime loop` component orchestrates a continuous, autonomous agent loop (a "ralph loop") based on the `adk-go` (https://github.com/google/adk-go) framework. 

To achieve a clean, modular, and beautiful configuration, the system models the continuous loop as an ordered sequence of self-contained `Stages`. Each Stage represents a distinct phase of production with a built-in feedback cycle: it prepares the environment, executes the core work, verifies the quality of the output, and finalizes the results. 

The configuration is defined in a Kubernetes Resource Model (KRM) multidoc YAML file containing a single `Loop` resource and multiple `Stage` resources.

## Goals

- Implement an autonomous, continuously repeating agent loop.
- Provide a crystal-clear, declarative KRM configuration that reads naturally: `Setup` -> `Work` -> `Verify` -> `Finalize`.
- Encapsulate the complex logic of rework, retries, and feedback loops cleanly within the `Verify` phase.
- Ensure robust execution semantics: explicit workspace sharing, configurable environment variables, and resilient top-level error handling.
- Automatically export rich metrics (durations, retry counts, pass/fail rates) out of the box, with support for custom metrics.
- Keep the architecture pristine so that an Operator could easily map these Stages to Kubernetes Jobs or Pods in the future.

## Non-Goals

- Human-in-the-loop approval. The loop is fully autonomous.
- Complex branching logic (e.g. DAGs, parallel stage execution). The loop remains a sequential pipeline of Stages.

## Design

### Configuration Model (KRM Multidoc)

The configuration is split into two KRM resources: `Loop` and `Stage`. 

The `Loop` is the orchestrator. It simply lists the `Stages` in the order they should be executed continuously.

```yaml
apiVersion: factory.ai.gke.io/v1alpha1
kind: Loop
metadata:
  name: feature-pipeline
spec:
  stages:
    - write-code
    - submit-pr
```

The `Stage` is the unit of production. It encapsulates a single, retryable feedback loop.

```yaml
apiVersion: factory.ai.gke.io/v1alpha1
kind: Stage
metadata:
  name: write-code
spec:
  setup:
    - tool:
        run: ["git", "checkout", "-b", "new-feature"]
        env:
          GIT_AUTHOR_NAME: "Ralph"
  
  work:
    - agent: "coder"
      prompt: "Implement the feature in main.go."
  
  verify:
    maxRetries: 3
    checks:
      - tool:
          run: ["go", "build", "main.go"]
        onFail: "The code failed to compile. Please fix the build errors."
      - tool:
          run: ["golangci-lint", "run"]
        allowFailure: true # Soft check (quality bar)
        onFail: "Linting failed. Please fix if possible, but not required."
  
  finalize:
    - tool:
        run: ["git", "add", "."]
    - tool:
        run: ["git", "commit", "-m", "Automated code implementation"]
```

### Execution Context & State

- **Shared Workspace**: All phases within a single Stage share the same execution workspace (current working directory). This allows `setup` to fetch code, `work` to modify it, and `finalize` to commit it.
- **Resilient Top-Level Loop**: If a Stage fatally errors (e.g., a tool crashes, or `verify` exhausts `maxRetries`), the current Loop iteration is immediately aborted. Because this is a continuous "Ralph loop", the engine will log the failure, back off, and restart the Loop from the first Stage. It does not crash the orchestrator.
- **Tool Schema**: Tools are robust. They accept a `run` array (command + args), an optional `cwd` (working directory), and an optional `env` map.

### The Anatomy of a Stage

A Stage executes its phases sequentially. The elegance of the Stage lies in its natural feedback loop during the `verify` phase.

1. **`setup`**: Executed exactly once at the beginning of the Stage. Used for environment preparation, branching, or fetching data.
2. **`work`**: The core execution phase. Contains a list of agents or tools that perform the actual task.
3. **`verify`**: The quality gate. It runs a list of `checks` (tools or agents). 
   - A check passes if its tool exits with code `0`, or its agent reports success.
   - If a check fails, its `allowFailure` flag is evaluated. If `allowFailure: true`, the error is logged as a warning, but the verify phase can still pass (this implements a soft "quality bar").
   - If any required check fails, the feedback (`onFail` prompt + the tool's error output) is sent back to the `work` phase, and the `work` phase is re-run. This feedback loop continues until the checks pass or `maxRetries` is exhausted.
4. **`finalize`**: Executed exactly once after a successful `verify` phase. Used for publishing, pushing PRs, or cleaning up.

### Execution Engine & Lifecycle

1. **Initialization**: The `factory runtime loop` command parses the multidoc YAML and validates the sequence.
2. **Continuous Loop**: The engine iterates through the `Loop`'s `spec.stages`. Once the final Stage is complete, it immediately restarts from the first Stage.
3. **Stage Execution**:
   - The engine runs `setup`.
   - The engine enters the **Work-Verify Cycle**.
   - It runs `work`.
   - It runs `verify`. If a required check fails, the engine increments the retry counter, injects the failure context into the next `work` run, and loops back. If `maxRetries` is reached without passing, the Stage halts with an error, aborting the current Loop iteration.
   - Upon successful verification, the engine runs `finalize`.
4. **Metrics**: The engine automatically emits metrics for every Stage transition. It tracks `stage_duration_seconds`, `verify_retry_count`, and `stage_success_total`. Tools and agents can also emit custom metrics to standard output which the engine will parse and export.

## Examples

### Scenario: The Ralph Loop

A continuous loop that constantly checks for issues and fixes them.

```yaml
apiVersion: factory.ai.gke.io/v1alpha1
kind: Loop
metadata:
  name: ralph
spec:
  stages:
    - triage-and-fix
---
apiVersion: factory.ai.gke.io/v1alpha1
kind: Stage
metadata:
  name: triage-and-fix
spec:
  work:
    - agent: "triage-bot"
      prompt: "Find the highest priority open issue and write a fix for it."
  verify:
    maxRetries: 5
    checks:
      - tool:
          run: ["make", "test"]
        onFail: "The fix broke the test suite. Analyze the failures and correct the implementation."
  finalize:
    - tool:
        run: ["gh", "pr", "create", "--fill"]
```

## Tests

### Unit Tests
- **`TestParseMultidocConfig`**: Verify the multidoc YAML gracefully unmarshals into the clean `Loop` and `Stage` Go structs.
- **`TestValidation`**: Verify that missing stages or invalid `maxRetries` values are caught immediately.
- **`TestWorkVerifyCycle`**: Use mocked agents and tool runners to verify the inner feedback loop: ensure `setup` runs once, `work` and `verify` iterate correctly up to `maxRetries`, and `finalize` only runs upon success.

### Integration Tests
- **`TestAutonomousRalphLoop`**: Run the loop in a sandbox. Simulate an environment where the first `work` attempt fails the `verify` check, but the subsequent retry passes, proving the feedback loop naturally self-corrects and proceeds to `finalize`.
