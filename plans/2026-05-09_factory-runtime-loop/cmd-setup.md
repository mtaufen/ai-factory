---
name: cmd-setup
---

Implement the CLI skeleton for `factory runtime loop` in `factory/cmd/factory/runtime/loop/loop.go`.
1. Register it in `factory/cmd/factory/runtime/runtime.go`.
2. Add a `--config` flag (required).
3. The command must verify that `GEMINI_API_KEY` is present in the environment.
4. It must parse the KRM manifests from the `--config` path using `api.Parse`.
5. Convert the unstructured objects to structured maps using `runtime.DefaultUnstructuredConverter`.
6. Locate the `api.Run` resource.
7. Return actionable error messages for any failures.
8. Add unit tests for this setup and flag parsing logic in `loop_test.go`, and add a basic sample KRM manifest at `testdata/sample-run.yaml` for testing.
