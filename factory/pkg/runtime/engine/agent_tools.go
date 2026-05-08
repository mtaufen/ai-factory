package engine

import (
	"context"
	"fmt"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

// GetAgentTools returns a list of allowed MCP tools and synthetic control tools
// for the given agent.
func GetAgentTools(ctx context.Context, agent *api.Agent, mcpManager mcp.ConnectionManager, servers map[string]*api.LocalMCPServer) ([]mcp.Tool, error) {
	var tools []mcp.Tool

	if agent.Spec.Tools != nil {
		for _, provider := range agent.Spec.Tools {
			if provider.MCP != nil {
				mcpTools, err := getMCPTools(ctx, provider.MCP, mcpManager, servers)
				if err != nil {
					return nil, err
				}
				tools = append(tools, mcpTools...)
			}
		}
	}

	// Add synthetic control tools
	tools = append(tools, mcp.Tool{
		Name:        "pass",
		Description: "Indicates the agent successfully completed its task.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Outcome message indicating success details",
				},
			},
			"required": []string{"message"},
		},
	})

	tools = append(tools, mcp.Tool{
		Name:        "fail",
		Description: "Indicates the agent encountered an unrecoverable error or could not make progress.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Outcome message indicating failure details",
				},
			},
			"required": []string{"message"},
		},
	})

	return tools, nil
}

func getMCPTools(ctx context.Context, config *api.MCPToolSet, mcpManager mcp.ConnectionManager, servers map[string]*api.LocalMCPServer) ([]mcp.Tool, error) {
	server, ok := servers[config.Name]
	if !ok {
		return nil, fmt.Errorf("local mcp server not found: %s", config.Name)
	}

	client, err := mcpManager.GetClient(ctx, config.Name, server.Spec.Pipe)
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp client: %w", err)
	}

	listRes, err := client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools for mcp server %s: %w", config.Name, err)
	}

	allowedNames := make(map[string]bool)
	for _, t := range config.Tools {
		allowedNames[t.Name] = true
	}

	var filtered []mcp.Tool
	for _, t := range listRes.Tools {
		if allowedNames[t.Name] {
			filtered = append(filtered, t)
		}
	}

	return filtered, nil
}

// CallAgentTool executes an allowed MCP tool or a synthetic control tool.
// Synthetic tools "pass" and "fail" return their message in the ToolResult.
func CallAgentTool(ctx context.Context, agent *api.Agent, mcpManager mcp.ConnectionManager, servers map[string]*api.LocalMCPServer, toolName string, args map[string]interface{}) (*mcp.ToolResult, error) {
	if toolName == "pass" || toolName == "fail" {
		msg, _ := args["message"].(string)
		return &mcp.ToolResult{
			Content: []mcp.ToolContent{
				{Type: "text", Text: fmt.Sprintf("%s: %s", toolName, msg)},
			},
		}, nil
	}

	if agent.Spec.Tools != nil {
		for _, provider := range agent.Spec.Tools {
			if provider.MCP != nil {
				for _, t := range provider.MCP.Tools {
					if t.Name == toolName {
						server, ok := servers[provider.MCP.Name]
						if !ok {
							return nil, fmt.Errorf("local mcp server not found: %s", provider.MCP.Name)
						}
						client, err := mcpManager.GetClient(ctx, provider.MCP.Name, server.Spec.Pipe)
						if err != nil {
							return nil, fmt.Errorf("failed to get mcp client: %w", err)
						}
						return client.CallTool(ctx, toolName, args)
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("tool %s not allowed or not found", toolName)
}
