package engine

import (
	"context"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

func TestGetAgentTools(t *testing.T) {
	ctx := context.Background()

	agent := &api.Agent{
		Spec: api.AgentSpec{
			Tools: []api.ToolProvider{
				{
					MCP: &api.MCPToolSet{
						Name: "my-mcp",
						Tools: []api.ToolConfig{
							{Name: "allowed-tool"},
						},
					},
				},
			},
		},
	}

	servers := map[string]*api.LocalMCPServer{
		"my-mcp": {
			Spec: api.LocalMCPServerSpec{
				Pipe: "dummy-pipe",
			},
		},
	}

	manager := &mockMCPConnectionManager{
		GetClientFunc: func(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
			return &mockMCPClient{
				ListToolsFunc: func(ctx context.Context) (*mcp.ListToolsResult, error) {
					return &mcp.ListToolsResult{
						Tools: []mcp.Tool{
							{Name: "allowed-tool", Description: "A allowed tool"},
							{Name: "hidden-tool", Description: "A hidden tool"},
						},
					}, nil
				},
			}, nil
		},
	}

	tools, err := GetAgentTools(ctx, agent, manager, servers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	// First should be "allowed-tool"
	if tools[0].Name != "allowed-tool" {
		t.Errorf("expected allowed-tool, got %s", tools[0].Name)
	}

	// Second should be "pass"
	if tools[1].Name != "pass" {
		t.Errorf("expected pass, got %s", tools[1].Name)
	}

	// Third should be "fail"
	if tools[2].Name != "fail" {
		t.Errorf("expected fail, got %s", tools[2].Name)
	}
}

func TestCallAgentTool(t *testing.T) {
	ctx := context.Background()

	agent := &api.Agent{
		Spec: api.AgentSpec{
			Tools: []api.ToolProvider{
				{
					MCP: &api.MCPToolSet{
						Name: "my-mcp",
						Tools: []api.ToolConfig{
							{Name: "allowed-tool"},
						},
					},
				},
			},
		},
	}

	servers := map[string]*api.LocalMCPServer{
		"my-mcp": {
			Spec: api.LocalMCPServerSpec{
				Pipe: "dummy-pipe",
			},
		},
	}

	manager := &mockMCPConnectionManager{
		GetClientFunc: func(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
			return &mockMCPClient{
				CallToolFunc: func(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error) {
					if name == "allowed-tool" {
						return &mcp.ToolResult{
							Content: []mcp.ToolContent{
								{Type: "text", Text: "tool executed"},
							},
						}, nil
					}
					return nil, nil
				},
			}, nil
		},
	}

	// Test synthetic tool "pass"
	res, err := CallAgentTool(ctx, agent, manager, servers, "pass", map[string]interface{}{"message": "done"})
	if err != nil {
		t.Fatalf("unexpected error for pass: %v", err)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "pass: done" {
		t.Errorf("unexpected pass result: %+v", res)
	}

	// Test synthetic tool "fail"
	res, err = CallAgentTool(ctx, agent, manager, servers, "fail", map[string]interface{}{"message": "broken"})
	if err != nil {
		t.Fatalf("unexpected error for fail: %v", err)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "fail: broken" {
		t.Errorf("unexpected fail result: %+v", res)
	}

	// Test allowed MCP tool
	res, err = CallAgentTool(ctx, agent, manager, servers, "allowed-tool", nil)
	if err != nil {
		t.Fatalf("unexpected error for allowed-tool: %v", err)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "tool executed" {
		t.Errorf("unexpected allowed-tool result: %+v", res)
	}

	// Test unlisted MCP tool
	_, err = CallAgentTool(ctx, agent, manager, servers, "hidden-tool", nil)
	if err == nil {
		t.Fatalf("expected error for hidden-tool, got nil")
	}
	if err.Error() != "tool hidden-tool not allowed or not found" {
		t.Errorf("unexpected error for hidden-tool: %v", err)
	}
}
