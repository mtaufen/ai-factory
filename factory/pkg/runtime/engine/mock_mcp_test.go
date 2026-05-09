package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/spf13/afero"
)

type readWriteCloser struct {
	io.Reader
	io.Writer
	close func() error
}

func (rwc *readWriteCloser) Close() error { return rwc.close() }

func createMockServerPipe() (clientEnd io.ReadWriteCloser, serverEnd io.ReadWriteCloser) {
	cRead, sWrite := io.Pipe()
	sRead, cWrite := io.Pipe()

	clientEnd = &readWriteCloser{
		Reader: cRead,
		Writer: cWrite,
		close: func() error {
			cRead.Close()
			cWrite.Close()
			return nil
		},
	}

	serverEnd = &readWriteCloser{
		Reader: sRead,
		Writer: sWrite,
		close: func() error {
			sRead.Close()
			sWrite.Close()
			return nil
		},
	}
	return
}

type mockPipeConnectionManager struct {
	clients map[string]mcp.Client
}

func (m *mockPipeConnectionManager) GetClient(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
	client, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	return client, nil
}

func (m *mockPipeConnectionManager) CloseAll() error {
	for _, c := range m.clients {
		c.Close()
	}
	return nil
}

func runDevMockMCPServer(t *testing.T, rw io.ReadWriteCloser, fs afero.Fs, tempDir string) {
	go func() {
		defer rw.Close()
		decoder := json.NewDecoder(rw)
		for {
			var req mcp.JSONRPCRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}

			var resBytes []byte

			if req.Method == "tools/list" {
				res := mcp.ListToolsResult{
					Tools: []mcp.Tool{
						{Name: "ReadFile", Description: "Reads a file"},
						{Name: "WriteFile", Description: "Writes a file"},
						{Name: "RunCommand", Description: "Runs a command"},
					},
				}
				resBytes, _ = json.Marshal(res)
			} else if req.Method == "tools/call" {
				paramsMap, ok := req.Params.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := paramsMap["name"].(string)
				argsMap, _ := paramsMap["arguments"].(map[string]interface{})

				var text string
				switch name {
				case "ReadFile":
					path, _ := argsMap["path"].(string)
					data, err := afero.ReadFile(fs, filepath.Join(tempDir, path))
					if err != nil {
						text = err.Error()
					} else {
						text = string(data)
					}
				case "WriteFile":
					path, _ := argsMap["path"].(string)
					content, _ := argsMap["content"].(string)
					err := afero.WriteFile(fs, filepath.Join(tempDir, path), []byte(content), 0644)
					if err != nil {
						text = err.Error()
					} else {
						text = "success"
					}
				case "RunCommand":
					text = "success"
				}

				tr := mcp.ToolResult{
					Content: []mcp.ToolContent{
						{Type: "text", Text: text},
					},
				}
				resBytes, _ = json.Marshal(tr)
			} else {
				errRes := mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &mcp.JSONRPCError{Code: -32601, Message: "Method not found: " + req.Method},
				}
				eb, _ := json.Marshal(errRes)
				eb = append(eb, '\n')
				rw.Write(eb)
				continue
			}

			resp := mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  resBytes,
			}
			b, _ := json.Marshal(resp)
			b = append(b, '\n')
			rw.Write(b)
		}
	}()
}

func runDummyMockMCPServer(t *testing.T, rw io.ReadWriteCloser) {
	go func() {
		defer rw.Close()
		decoder := json.NewDecoder(rw)
		for {
			var req mcp.JSONRPCRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}

			var resBytes []byte

			if req.Method == "tools/list" {
				res := mcp.ListToolsResult{
					Tools: []mcp.Tool{
						{Name: "git-commit", Description: "Commits files"},
					},
				}
				resBytes, _ = json.Marshal(res)
			} else if req.Method == "tools/call" {
				tr := mcp.ToolResult{
					Content: []mcp.ToolContent{
						{Type: "text", Text: "git commit success"},
					},
				}
				resBytes, _ = json.Marshal(tr)
			} else {
				continue
			}

			resp := mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  resBytes,
			}
			b, _ := json.Marshal(resp)
			b = append(b, '\n')
			rw.Write(b)
		}
	}()
}
