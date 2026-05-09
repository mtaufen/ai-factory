package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/engine"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/history"
	"github.com/spf13/afero"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

// --- Copied from mock_mcp_test.go because we cannot import _test.go files ---

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
						{
							Name: "ReadFile", Description: "Reads a file",
							InputSchema: map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"path": map[string]interface{}{"type": "string"},
								},
								"required": []string{"path"},
							},
						},
						{
							Name: "WriteFile", Description: "Writes a file",
							InputSchema: map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"path":    map[string]interface{}{"type": "string"},
									"content": map[string]interface{}{"type": "string"},
								},
								"required": []string{"path", "content"},
							},
						},
						{
							Name: "RunCommand", Description: "Runs a command",
							InputSchema: map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"command": map[string]interface{}{"type": "string"},
								},
								"required": []string{"command"},
							},
						},
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

// --- End Copied ---

func TestLoopLiveExecution(t *testing.T) {
	// 1. Skip if not set
	if os.Getenv("FACTORY_LIVE_TEST") != "true" || os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("Skipping live test: FACTORY_LIVE_TEST=true and GEMINI_API_KEY must be set in environment.")
	}

	// 3. Read and parse manifests
	configPath := "testdata/sample-run.yaml"
	file, err := os.Open(configPath)
	if err != nil {
		t.Fatalf("failed to open config: %v", err)
	}
	defer file.Close()

	unstructuredObjs, err := api.Parse(file)
	if err != nil {
		t.Fatalf("failed to parse manifests: %v", err)
	}

	runs := make(map[string]*api.Run)
	loops := make(map[string]*api.Loop)
	agents := make(map[string]*api.Agent)
	servers := make(map[string]*api.LocalMCPServer)

	for _, u := range unstructuredObjs {
		switch u.GetKind() {
		case "Run":
			var run api.Run
			if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &run); err != nil {
				t.Fatalf("Failed to convert Run: %v", err)
			}
			runs[run.Name] = &run
		case "Loop":
			var loop api.Loop
			if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &loop); err != nil {
				t.Fatalf("Failed to convert Loop: %v", err)
			}
			loops[loop.Name] = &loop
		case "Agent":
			var agent api.Agent
			if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &agent); err != nil {
				t.Fatalf("Failed to convert Agent: %v", err)
			}
			agents[agent.Name] = &agent
		case "LocalMCPServer":
			var server api.LocalMCPServer
			if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &server); err != nil {
				t.Fatalf("Failed to convert LocalMCPServer: %v", err)
			}
			servers[server.Name] = &server
		}
	}

	var run *api.Run
	for _, r := range runs {
		run = r
		break
	}
	if run == nil {
		t.Fatal("no Run resource found in manifests")
	}

	// 4. Set up afero and Mock MCP Server
	fs := afero.NewMemMapFs()
	tempDir := "/workspace"
	fs.MkdirAll(tempDir, 0755)

	clientEnd, serverEnd := createMockServerPipe()
	mcpClient := mcp.NewClientWithPipe(clientEnd)
	
	mcpManager := &mockPipeConnectionManager{
		clients: map[string]mcp.Client{
			"my-mcp-server": mcpClient,
		},
	}

	runDevMockMCPServer(t, serverEnd, fs, tempDir)

	// 5. Initialize the real Gemini model
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	apiKey := os.Getenv("GEMINI_API_KEY")
	clientConfig := &genai.ClientConfig{APIKey: apiKey}
	geminiModel, err := gemini.NewModel(ctx, "gemini-3.1-pro-preview", clientConfig)
	if err != nil {
		t.Fatalf("failed to initialize gemini model: %v", err)
	}

	llmClient := history.NewADKModelAdapter(geminiModel)
	summarizer := history.NewLLMSummarizer(llmClient)

	agentExecutor := engine.NewAgentExecutor(agents, mcpManager, servers, geminiModel)
	mcpExecutor := engine.NewMCPStepExecutor(mcpManager, servers)
	stdExecutor := &noopExecutor{}

	runner := &engine.Runner{
		Run:           run,
		Loops:         loops,
		Executor:      stdExecutor,
		MCPExecutor:   mcpExecutor,
		AgentExecutor: agentExecutor,
		Summarizer:    summarizer,
	}

	// 6. Execute Loop
	pass, msg, _, err := runner.ExecuteLoop(ctx, run.Spec.Start, nil)
	if err != nil {
		t.Fatalf("Execution failed with error: %v", err)
	}
	if !pass {
		t.Fatalf("Execution failed: %v", msg)
	}

	// 7. Verify Side Effects on Afero Filesystem
	data, err := afero.ReadFile(fs, filepath.Join(tempDir, "output.txt"))
	if err != nil {
		t.Fatalf("Expected output.txt to be written, but got error: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "hello") && content != "hello" {
		t.Errorf("Expected output.txt to contain 'hello', got '%s'", content)
	}
}
