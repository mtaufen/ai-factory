package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/history"
)

type mockLLMClient struct {
	responses []mockResponse
	callCount int
	t         *testing.T
}

type mockResponse struct {
	toolCall string
	toolArgs map[string]interface{}
	err      error
}

func (m *mockLLMClient) Call(ctx context.Context, prompt string, history history.History, tools []mcp.Tool) (string, map[string]interface{}, error) {
	if m.callCount >= len(m.responses) {
		m.t.Fatalf("unexpected call to LLM")
	}
	res := m.responses[m.callCount]
	m.callCount++
	return res.toolCall, res.toolArgs, res.err
}

func TestAgentExecutor(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "agent.md")
	os.WriteFile(promptPath, []byte("Base prompt"), 0644)

	agents := map[string]*api.Agent{
		"test-agent": {
			Spec: api.AgentSpec{
				Path: promptPath,
				Tools: []api.ToolProvider{
					{
						MCP: &api.MCPToolSet{
							Name: "test-server",
							Tools: []api.ToolConfig{
								{Name: "readFile"},
							},
						},
					},
				},
			},
		},
	}

	servers := map[string]*api.LocalMCPServer{
		"test-server": {
			Spec: api.LocalMCPServerSpec{
				Pipe: "dummy",
			},
		},
	}

	step := &api.Step{
		Name: "test-step",
		Agent: &api.AgentAction{
			Name:   "test-agent",
			Prompt: "Inline prompt $(IDEA)",
			Args: []api.Argument{
				{Name: "IDEA", Value: "test-idea"},
			},
		},
	}

	// We are going to mock MCPManager but for AgentExecutor testing we only need
	// GetAgentTools to succeed. However, GetAgentTools itself calls mcpManager to list tools.
	// Since GetAgentTools relies on the MCPManager to query allowed tools, we can mock LLM
	// to just return pass/fail and test prompt assembly and history.

	manager := &mockMCPConnectionManager{
		GetClientFunc: func(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
			return &mockMCPClient{
				ListToolsFunc: func(ctx context.Context) (*mcp.ListToolsResult, error) {
					return &mcp.ListToolsResult{
						Tools: []mcp.Tool{
							{Name: "readFile", Description: "Reads a file"},
							{Name: "hiddenTool", Description: "Not allowed"},
						},
					}, nil
				},
				CallToolFunc: func(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error) {
					return &mcp.ToolResult{
						Content: []mcp.ToolContent{
							{Type: "text", Text: "File content"},
						},
					}, nil
				},
			}, nil
		},
	}

	llm := &mockLLMClient{
		t: t,
		responses: []mockResponse{
			{
				toolCall: "readFile",
				toolArgs: map[string]interface{}{"path": "foo.txt"},
			},
			{
				toolCall: "pass",
				toolArgs: map[string]interface{}{"message": "done"},
			},
		},
	}

	executor := NewAgentExecutor(agents, manager, servers, llm)

	pass, msg, h, err := executor.Execute(context.Background(), step, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Errorf("expected pass=true")
	}
	if msg != "done" {
		t.Errorf("expected msg='done', got '%s'", msg)
	}
	if len(h) != 1 {
		t.Errorf("expected history length 1 (from tool call), got %d", len(h))
	}
}
