package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/spf13/afero"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestLoopPodIntegration(t *testing.T) {
	tempDir := t.TempDir()
	fs := afero.NewMemMapFs()

	devClientEnd, devServerEnd := createMockServerPipe()
	gitClientEnd, gitServerEnd := createMockServerPipe()

	runDevMockMCPServer(t, devServerEnd, fs, tempDir)
	runDummyMockMCPServer(t, gitServerEnd)

	devClient := mcp.NewClientWithPipe(devClientEnd)
	gitClient := mcp.NewClientWithPipe(gitClientEnd)

	promptPath := filepath.Join(tempDir, "agent.md")
	os.WriteFile(promptPath, []byte("Integration test agent prompt"), 0644)

	agents := map[string]*api.Agent{
		"test-agent": {
			Spec: api.AgentSpec{
				Prompt: &api.PromptSource{
					Path: promptPath,
				},
				Tools: []api.ToolProvider{
					{
						MCP: &api.MCPToolSet{
							Name: "dev-server",
							Tools: []api.ToolConfig{
								{Name: "WriteFile"},
								{Name: "ReadFile"},
								{Name: "RunCommand"},
							},
						},
					},
					{
						MCP: &api.MCPToolSet{
							Name: "git-server",
							Tools: []api.ToolConfig{
								{Name: "git-commit"},
							},
						},
					},
				},
			},
		},
	}

	servers := map[string]*api.LocalMCPServer{
		"dev-server": {
			Spec: api.LocalMCPServerSpec{
				Pipe: "dev",
			},
		},
		"git-server": {
			Spec: api.LocalMCPServerSpec{
				Pipe: "git",
			},
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
								Name: "WriteFile",
								Args: map[string]any{"path": "hello.txt", "content": "world"},
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
								Name: "ReadFile",
								Args: map[string]any{"path": "hello.txt"},
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
								Name: "RunCommand",
								Args: map[string]any{"command": "echo hello"},
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
								Name: "git-commit",
								Args: map[string]any{"message": "initial commit"},
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
								Args: map[string]any{"message": "integration test complete"},
							},
						},
					},
				},
			},
		},
	}

	manager := &mockPipeConnectionManager{
		clients: map[string]mcp.Client{
			"dev-server": devClient,
			"git-server": gitClient,
		},
	}
	defer manager.CloseAll()

	executor := NewAgentExecutor(agents, manager, servers, llm)

	step := &api.Step{
		Name: "integration-step",
		Agent: &api.AgentAction{
			Name:   "test-agent",
			Prompt: "Do the work",
		},
	}

	pass, msg, history, err := executor.Execute(context.Background(), step, map[string]string{}, nil)

	if err != nil {
		t.Fatalf("unexpected error executing agent: %v", err)
	}

	if !pass {
		t.Errorf("expected pass to be true")
	}

	if msg != "integration test complete" {
		t.Errorf("expected msg 'integration test complete', got '%s'", msg)
	}

	if len(history) == 0 {
		t.Errorf("expected non-empty history")
	}

	data, err := afero.ReadFile(fs, filepath.Join(tempDir, "hello.txt"))
	if err != nil {
		t.Fatalf("expected hello.txt to exist: %v", err)
	}
	if string(data) != "world" {
		t.Errorf("expected hello.txt to contain 'world', got '%s'", string(data))
	}
}
