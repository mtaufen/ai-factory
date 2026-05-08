package engine

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type mockADKModel struct {
	responses []*model.LLMResponse
	errs      []error
	callCount int
	t         *testing.T
}

func (m *mockADKModel) Name() string {
	return "mock-adk-model"
}

func (m *mockADKModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if m.callCount >= len(m.responses) {
			m.t.Fatalf("unexpected call to GenerateContent, callCount=%d", m.callCount)
			return
		}

		res := m.responses[m.callCount]
		var err error
		if len(m.errs) > m.callCount {
			err = m.errs[m.callCount]
		}
		m.callCount++

		yield(res, err)
	}
}

func TestAgentExecutor(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "agent.md")
	os.WriteFile(promptPath, []byte("Base prompt"), 0644)

	agents := map[string]*api.Agent{
		"test-agent": {
			Spec: api.AgentSpec{
				Prompt: &api.PromptSource{
					Path: promptPath,
				},
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

	llm := &mockADKModel{
		t: t,
		responses: []*model.LLMResponse{
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{
							FunctionCall: &genai.FunctionCall{
								Name: "readFile",
								Args: map[string]any{"path": "foo.txt"},
							},
						},
					},
				},
			},
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{
							FunctionCall: &genai.FunctionCall{
								Name: "pass",
								Args: map[string]any{"message": "done"},
							},
						},
					},
				},
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
	if len(h) == 0 {
		t.Errorf("expected non-empty history")
	}
}
