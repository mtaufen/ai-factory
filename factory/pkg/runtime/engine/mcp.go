package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/history"
)

// MCPStepExecutor executes MCP steps.
type MCPStepExecutor struct {
	Manager mcp.ConnectionManager
	Servers map[string]*api.LocalMCPServer
}

func NewMCPStepExecutor(manager mcp.ConnectionManager, servers map[string]*api.LocalMCPServer) *MCPStepExecutor {
	return &MCPStepExecutor{
		Manager: manager,
		Servers: servers,
	}
}

func (e *MCPStepExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string, currentHistory history.History) (bool, string, history.History, error) {
	if step.MCP == nil {
		return false, "", nil, fmt.Errorf("step %s is not an MCP step", step.Name)
	}

	server, ok := e.Servers[step.MCP.Name]
	if !ok {
		return false, "", nil, fmt.Errorf("local mcp server not found: %s", step.MCP.Name)
	}

	client, err := e.Manager.GetClient(ctx, step.MCP.Name, server.Spec.Pipe)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to get mcp client: %w", err)
	}

	toolArgs := make(map[string]interface{})
	for _, arg := range step.MCP.Args {
		val := api.ExpandVariables(arg.Value, args)
		toolArgs[arg.Name] = val
	}

	res, err := client.CallTool(ctx, step.MCP.Tool, toolArgs)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to call mcp tool: %w", err)
	}

	var msgs []string
	for _, c := range res.Content {
		msgs = append(msgs, c.Text)
	}
	msg := strings.Join(msgs, "\n")

	if res.IsError {
		return false, msg, history.History{{Role: "system", Content: msg}}, nil
	}

	return true, msg, history.History{{Role: "system", Content: msg}}, nil
}
