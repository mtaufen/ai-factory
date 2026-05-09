---
name: loop-live-test
---

Create a live integration test in `factory/cmd/factory/runtime/loop/loop_live_test.go`.
1. Attempt to load environment variables from the gitignored `.env/.env` file at the project root.
2. The test should skip unless both `FACTORY_LIVE_TEST=true` and `GEMINI_API_KEY` are set in the environment.
3. It should reuse the `afero`-backed mock MCP servers and pipe generation from `mock_mcp_test.go` to test locally without needing remote networks.
4. Test against the static YAML manifest at `testdata/sample-run.yaml`.
5. Point `LocalMCPServer` definitions in the test at ephemeral pipes and initialize the real Gemini LLM client.
6. Execute the loop and verify the expected side effects on the `afero` filesystem.
