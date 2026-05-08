package engine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/history"
)

// AgentExecutorImpl executes Agent steps.
type AgentExecutorImpl struct {
	Agents     map[string]*api.Agent
	MCPManager mcp.ConnectionManager
	Servers    map[string]*api.LocalMCPServer
	LLMClient  LLMMockClient // Mockable LLM client for tests
}

func NewAgentExecutor(agents map[string]*api.Agent, manager mcp.ConnectionManager, servers map[string]*api.LocalMCPServer, llmClient LLMMockClient) *AgentExecutorImpl {
	return &AgentExecutorImpl{
		Agents:     agents,
		MCPManager: manager,
		Servers:    servers,
		LLMClient:  llmClient,
	}
}

// LLMMockClient is an interface to mock the LLM interaction since adk-go
// requires a live model for testing in many cases, or its own internal mocks.
type LLMMockClient interface {
	// Call represents a single turn in the LLM loop
	Call(ctx context.Context, prompt string, history history.History, tools []mcp.Tool) (toolCall string, args map[string]interface{}, err error)
}

func (e *AgentExecutorImpl) Execute(ctx context.Context, step *api.Step, args map[string]string, currentHistory history.History) (bool, string, history.History, error) {
	if step.Agent == nil {
		return false, "", nil, fmt.Errorf("step %s is not an agent step", step.Name)
	}

	agentDef, ok := e.Agents[step.Agent.Name]
	if !ok {
		return false, "", nil, fmt.Errorf("agent not found: %s", step.Agent.Name)
	}

	var prompt string
	if agentDef.Spec.Path != "" {
		promptBytes, err := os.ReadFile(agentDef.Spec.Path)
		if err != nil {
			return false, "", nil, fmt.Errorf("failed to read agent prompt: %w", err)
		}
		prompt = string(promptBytes)
	}

	if step.Agent.Prompt != "" {
		if prompt != "" {
			prompt += "\n"
		}
		prompt += step.Agent.Prompt
	}

	// Interpolate args into the prompt. Agents as tools pattern.
	for _, arg := range step.Agent.Args {
		val := api.ExpandVariables(arg.Value, args)
		prompt = strings.ReplaceAll(prompt, fmt.Sprintf("$(%s)", arg.Name), val)
	}

	tools, err := GetAgentTools(ctx, agentDef, e.MCPManager, e.Servers)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to get agent tools: %w", err)
	}

	maxTurns := 10
	turns := 0
	agentHistory := append(history.History{}, currentHistory...)

	// Execute the agent's LLM loop.
	// In a real implementation we would use adk-go here.
	// We simulate the adk-go agent loop using the mockable client to allow testing
	// loop termination, tool injection, and history passing.
	for turns < maxTurns {
		turns++

		toolCall, toolArgs, err := e.LLMClient.Call(ctx, prompt, agentHistory, tools)
		if err != nil {
			return false, "", nil, fmt.Errorf("llm error: %w", err)
		}

		// Agent yielded control back via pass or fail
		if toolCall == "pass" {
			msg, _ := toolArgs["message"].(string)
			return true, msg, agentHistory, nil
		}
		if toolCall == "fail" {
			msg, _ := toolArgs["message"].(string)
			return false, msg, agentHistory, nil
		}

		// Call the MCP tool
		res, err := CallAgentTool(ctx, agentDef, e.MCPManager, e.Servers, toolCall, toolArgs)
		if err != nil {
			// Pass the error back to the LLM to recover
			agentHistory = append(agentHistory, history.Message{
				Role:    "tool",
				Content: fmt.Sprintf("error: %v", err),
			})
			continue
		}

		var toolOutputs []string
		for _, c := range res.Content {
			toolOutputs = append(toolOutputs, c.Text)
		}

		agentHistory = append(agentHistory, history.Message{
			Role:    "tool",
			Content: strings.Join(toolOutputs, "\n"),
		})
	}

	return false, "agent exceeded maximum interaction turns", agentHistory, nil
}
