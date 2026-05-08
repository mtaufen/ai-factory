package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"google.golang.org/adk/model"
)

func TestGetADKTools(t *testing.T) {
	ctx := context.Background()

	agent := &api.Agent{
		Spec: api.AgentSpec{
			Tools: []api.ToolProvider{
				{
					MCP: &api.MCPToolSet{
						Name: "test-mcp",
						Tools: []api.ToolConfig{
							{Name: "read_file"},
						},
					},
				},
			},
		},
	}

	servers := map[string]*api.LocalMCPServer{
		"test-mcp": {
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
							{
								Name:        "read_file",
								Description: "Reads a file",
								InputSchema: map[string]interface{}{
									"type": "object",
								},
							},
							{
								Name:        "ignored_tool",
								Description: "Ignored tool",
							},
						},
					}, nil
				},
			}, nil
		},
	}

	var outcome ControlOutcome
	tools, err := GetADKTools(ctx, agent, manager, servers, &outcome)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	t1 := tools[0]
	if t1.Name() != "read_file" {
		t.Errorf("expected read_file, got %s", t1.Name())
	}
	if t1.Description() != "Reads a file" {
		t.Errorf("expected description 'Reads a file', got %q", t1.Description())
	}
	if t1.IsLongRunning() {
		t.Errorf("expected IsLongRunning false")
	}
	adapter, ok := t1.(*MCPToolAdapter)
	if !ok {
		t.Fatalf("expected tool to be *MCPToolAdapter")
	}
	decl := adapter.Declaration()
	if decl == nil || decl.Name != "read_file" {
		t.Errorf("unexpected declaration: %+v", decl)
	}

	req := &model.LLMRequest{}
	if err := adapter.ProcessRequest(nil, req); err != nil {
		t.Errorf("unexpected error processing request: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Errorf("expected 1 packed tool")
	}

	t2 := tools[1]
	if t2.Name() != "pass" {
		t.Errorf("expected pass, got %s", t2.Name())
	}

	t3 := tools[2]
	if t3.Name() != "fail" {
		t.Errorf("expected fail, got %s", t3.Name())
	}
}

func TestMCPToolAdapter_Run(t *testing.T) {
	agent := &api.Agent{
		Spec: api.AgentSpec{
			Tools: []api.ToolProvider{
				{
					MCP: &api.MCPToolSet{
						Name: "test-mcp",
						Tools: []api.ToolConfig{
							{Name: "echo"},
						},
					},
				},
			},
		},
	}

	servers := map[string]*api.LocalMCPServer{
		"test-mcp": {},
	}

	manager := &mockMCPConnectionManager{
		GetClientFunc: func(ctx context.Context, name string, pipePath string) (mcp.Client, error) {
			return &mockMCPClient{
				CallToolFunc: func(ctx context.Context, name string, args map[string]interface{}) (*mcp.ToolResult, error) {
					if name == "echo" {
						msg, _ := args["text"].(string)
						if msg == "error-trigger" {
							return &mcp.ToolResult{
								IsError: true,
								Content: []mcp.ToolContent{
									{Type: "text", Text: "something went wrong"},
								},
							}, nil
						}
						return &mcp.ToolResult{
							Content: []mcp.ToolContent{
								{Type: "text", Text: "echo: " + msg},
							},
						}, nil
					}
					return nil, errors.New("unknown tool")
				},
			}, nil
		},
	}

	adapter := &MCPToolAdapter{
		toolSchema: mcp.Tool{Name: "echo"},
		agent:      agent,
		mcpManager: manager,
		servers:    servers,
	}

	res, err := adapter.Run(nil, map[string]interface{}{"text": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out, ok := res["output"].(string); !ok || out != "echo: hello" {
		t.Errorf("unexpected output: %+v", res)
	}

	_, err = adapter.Run(nil, map[string]interface{}{"text": "error-trigger"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSyntheticControlTool_Run(t *testing.T) {
	var outcome ControlOutcome

	passTool := &SyntheticControlTool{
		name:        "pass",
		description: "Pass tool",
		pass:        true,
		outcome:     &outcome,
	}

	res, err := passTool.Run(nil, map[string]interface{}{"message": "all tests passed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r, ok := res["result"].(string); !ok || r != "pass: all tests passed" {
		t.Errorf("unexpected result: %+v", res)
	}

	if !outcome.Called || !outcome.Pass || outcome.Message != "all tests passed" {
		t.Errorf("unexpected outcome state: %+v", outcome)
	}

	var outcome2 ControlOutcome
	failTool := &SyntheticControlTool{
		name:        "fail",
		description: "Fail tool",
		pass:        false,
		outcome:     &outcome2,
	}

	_, err = failTool.Run(nil, map[string]any{"message": "fatal error"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !outcome2.Called || outcome2.Pass || outcome2.Message != "fatal error" {
		t.Errorf("unexpected outcome state: %+v", outcome2)
	}
}
