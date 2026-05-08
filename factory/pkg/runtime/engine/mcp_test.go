package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

type mockMCPClient struct {
	CallToolFunc  func(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error)
	ListToolsFunc func(ctx context.Context) (*mcp.ListToolsResult, error)
}

func (m *mockMCPClient) Connect(ctx context.Context) error {
	return nil
}

func (m *mockMCPClient) Close() error {
	return nil
}

func (m *mockMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error) {
	if m.CallToolFunc != nil {
		return m.CallToolFunc(ctx, name, args)
	}
	return &mcp.ToolResult{}, nil
}

func (m *mockMCPClient) ListTools(ctx context.Context) (*mcp.ListToolsResult, error) {
	if m.ListToolsFunc != nil {
		return m.ListToolsFunc(ctx)
	}
	return &mcp.ListToolsResult{}, nil
}

type mockMCPConnectionManager struct {
	GetClientFunc func(ctx context.Context, name string, pipePath string) (mcp.Client, error)
}

func (m *mockMCPConnectionManager) GetClient(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
	if m.GetClientFunc != nil {
		return m.GetClientFunc(ctx, name, pipePath)
	}
	return &mockMCPClient{}, nil
}

func (m *mockMCPConnectionManager) CloseAll() error {
	return nil
}

func TestMCPStepExecutor_Execute(t *testing.T) {
	servers := map[string]*api.LocalMCPServer{
		"git-mcp": {
			Spec: api.LocalMCPServerSpec{
				Pipe: "/var/run/mcp/git-mcp",
			},
		},
	}

	tests := []struct {
		name          string
		step          *api.Step
		args          map[string]string
		getClientFunc func(ctx context.Context, name string, pipePath string) (mcp.Client, error)
		wantPass      bool
		wantMsg       string
		wantErr       string
	}{
		{
			name: "Not MCP Step",
			step: &api.Step{
				Name: "step1",
			},
			wantErr: "step step1 is not an MCP step",
		},
		{
			name: "Server Not Found",
			step: &api.Step{
				Name: "step1",
				MCP: &api.MCPAction{
					Name: "missing-mcp",
					Tool: "clone_repository",
				},
			},
			wantErr: "local mcp server not found: missing-mcp",
		},
		{
			name: "Manager Error",
			step: &api.Step{
				Name: "step1",
				MCP: &api.MCPAction{
					Name: "git-mcp",
					Tool: "clone_repository",
				},
			},
			getClientFunc: func(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
				return nil, errors.New("connection failed")
			},
			wantErr: "failed to get mcp client: connection failed",
		},
		{
			name: "Tool Error",
			step: &api.Step{
				Name: "step1",
				MCP: &api.MCPAction{
					Name: "git-mcp",
					Tool: "clone_repository",
				},
			},
			getClientFunc: func(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
				return &mockMCPClient{
					CallToolFunc: func(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error) {
						return nil, errors.New("tool call failed")
					},
				}, nil
			},
			wantErr: "failed to call mcp tool: tool call failed",
		},
		{
			name: "Tool Returned Error",
			step: &api.Step{
				Name: "step1",
				MCP: &api.MCPAction{
					Name: "git-mcp",
					Tool: "clone_repository",
				},
			},
			getClientFunc: func(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
				return &mockMCPClient{
					CallToolFunc: func(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error) {
						return &mcp.ToolResult{
							Content: []mcp.ToolContent{
								{Type: "text", Text: "repo not found"},
							},
							IsError: true,
						}, nil
					},
				}, nil
			},
			wantPass: false,
			wantMsg:  "repo not found",
		},
		{
			name: "Success with Interpolation",
			step: &api.Step{
				Name: "step1",
				MCP: &api.MCPAction{
					Name: "git-mcp",
					Tool: "clone_repository",
					Args: []api.Argument{
						{Name: "remote", Value: "$(REPO_URL)"},
					},
				},
			},
			args: map[string]string{
				"REPO_URL": "https://github.com/example/repo",
			},
			getClientFunc: func(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
				if pipePath != "/var/run/mcp/git-mcp" {
					t.Errorf("expected pipePath /var/run/mcp/git-mcp, got %s", pipePath)
				}
				return &mockMCPClient{
					CallToolFunc: func(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error) {
						if name != "clone_repository" {
							t.Errorf("expected tool clone_repository, got %s", name)
						}
						if args["remote"] != "https://github.com/example/repo" {
							t.Errorf("expected remote arg https://github.com/example/repo, got %v", args["remote"])
						}
						return &mcp.ToolResult{
							Content: []mcp.ToolContent{
								{Type: "text", Text: "cloned successfully"},
							},
						}, nil
					},
				}, nil
			},
			wantPass: true,
			wantMsg:  "cloned successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &mockMCPConnectionManager{
				GetClientFunc: tt.getClientFunc,
			}
			executor := NewMCPStepExecutor(manager, servers)

			pass, msg, history, err := executor.Execute(context.Background(), tt.step, tt.args, nil)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pass != tt.wantPass {
				t.Errorf("expected pass %v, got %v", tt.wantPass, pass)
			}

			if msg != tt.wantMsg {
				t.Errorf("expected msg %q, got %q", tt.wantMsg, msg)
			}

			if len(history) != 1 || history[0].Content != tt.wantMsg {
				t.Errorf("expected history content %q, got %v", tt.wantMsg, history)
			}
		})
	}
}
