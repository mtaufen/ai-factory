---
name: mock-mcp-afero
---

Update `factory/pkg/runtime/engine/mock_mcp_test.go` to use `github.com/spf13/afero` instead of the OS filesystem.
1. Add `afero.Fs` to the test server functions (like `runDevMockMCPServer`).
2. Replace `os.ReadFile` and `os.WriteFile` with operations on the `afero.Fs`.
3. Ensure any calling tests (e.g. `integration_test.go`) instantiate an `afero.NewMemMapFs()` and pass it to the mock servers.
You may need to run `go get github.com/spf13/afero` and `go mod tidy`.
