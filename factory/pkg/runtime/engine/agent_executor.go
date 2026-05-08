package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/history"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// AgentExecutorImpl executes Agent steps.
type AgentExecutorImpl struct {
	Agents     map[string]*api.Agent
	MCPManager mcp.ConnectionManager
	Servers    map[string]*api.LocalMCPServer
	LLMClient  model.LLM // Mockable ADK model interface for tests
}

func NewAgentExecutor(agents map[string]*api.Agent, manager mcp.ConnectionManager, servers map[string]*api.LocalMCPServer, llmClient model.LLM) *AgentExecutorImpl {
	return &AgentExecutorImpl{
		Agents:     agents,
		MCPManager: manager,
		Servers:    servers,
		LLMClient:  llmClient,
	}
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
	if agentDef.Spec.Prompt != nil {
		if agentDef.Spec.Prompt.Value != "" {
			prompt = agentDef.Spec.Prompt.Value
		} else if agentDef.Spec.Prompt.Path != "" {
			promptBytes, err := os.ReadFile(agentDef.Spec.Prompt.Path)
			if err != nil {
				return false, "", nil, fmt.Errorf("failed to read agent prompt: %w", err)
			}
			prompt = string(promptBytes)
		}
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

	var outcome ControlOutcome
	adkTools, err := GetADKTools(ctx, agentDef, e.MCPManager, e.Servers, &outcome)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to get adk tools: %w", err)
	}

	var genCfg *genai.GenerateContentConfig
	if agentDef.Spec.Temperature != nil || agentDef.Spec.MaxTokens != nil {
		genCfg = &genai.GenerateContentConfig{}
		if agentDef.Spec.Temperature != nil {
			genCfg.Temperature = agentDef.Spec.Temperature
		}
		if agentDef.Spec.MaxTokens != nil {
			genCfg.MaxOutputTokens = *agentDef.Spec.MaxTokens
		}
	}

	agentName := step.Agent.Name
	if agentDef.Name != "" {
		agentName = agentDef.Name
	}

	adkAgent, err := llmagent.New(llmagent.Config{
		Name:                  agentName,
		Description:           "Loop agent",
		Model:                 e.LLMClient,
		Instruction:           prompt,
		Tools:                 adkTools,
		GenerateContentConfig: genCfg,
	})
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to create adk agent: %w", err)
	}

	sessSvc := session.InMemoryService()
	createResp, err := sessSvc.Create(ctx, &session.CreateRequest{
		AppName:   "ai-factory",
		UserID:    "user",
		SessionID: "loop-session",
	})
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to create session: %w", err)
	}
	storedSession := createResp.Session

	// Map existing currentHistory to storedSession
	for _, hm := range currentHistory {
		ev := session.NewEvent("init")
		ev.Author = agentName
		if hm.Role == "user" {
			ev.Author = "user"
		}
		ev.LLMResponse.Content = &genai.Content{
			Role:  hm.Role,
			Parts: []*genai.Part{{Text: hm.Content}},
		}
		_ = sessSvc.AppendEvent(ctx, storedSession, ev)
	}

	r, err := runner.New(runner.Config{
		AppName:        "ai-factory",
		Agent:          adkAgent,
		SessionService: sessSvc,
	})
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to create runner: %w", err)
	}

	// Execute the agent
	seq := r.Run(ctx, "user", "loop-session", nil, agent.RunConfig{})

	var lastErr error
	for ev, err := range seq {
		if err != nil {
			lastErr = err
		}
		_ = ev
	}

	if lastErr != nil && !outcome.Called {
		return false, "", nil, fmt.Errorf("llm error: %w", lastErr)
	}

	agentHistory := append(history.History{}, currentHistory...)

	// Extract new events from session mapping them back to history.Message objects
	getResp, err := sessSvc.Get(ctx, &session.GetRequest{
		AppName:   "ai-factory",
		UserID:    "user",
		SessionID: "loop-session",
	})
	if err == nil && getResp.Session != nil {
		storedSession = getResp.Session
	}
	events := storedSession.Events()
	for i := 0; i < events.Len(); i++ {
		ev := events.At(i)
		if ev.InvocationID == "init" {
			continue
		}

		var content *genai.Content
		if ev.Content != nil {
			content = ev.Content
		} else if ev.LLMResponse.Content != nil {
			content = ev.LLMResponse.Content
		}

		if content != nil {
			var sb strings.Builder
			role := "model"

			if ev.Author == "user" {
				role = "user"
			}

			for _, part := range content.Parts {
				if part.Text != "" {
					sb.WriteString(part.Text)
				} else if part.FunctionCall != nil {
					sb.WriteString(fmt.Sprintf("call: %s", part.FunctionCall.Name))
				} else if part.FunctionResponse != nil {
					role = "tool"
					m := part.FunctionResponse.Response
					if out, ok := m["output"].(string); ok {
						sb.WriteString(out)
					} else if res, ok := m["result"].(string); ok {
						sb.WriteString(res)
					} else {
						b, _ := json.Marshal(m)
						sb.WriteString(string(b))
					}
				}
			}

			if sb.Len() > 0 {
				agentHistory = append(agentHistory, history.Message{
					Role:    role,
					Content: sb.String(),
				})
			}
		}
	}

	if outcome.Called {
		return outcome.Pass, outcome.Message, agentHistory, nil
	}

	return false, "agent exceeded maximum interaction turns", agentHistory, nil
}
