package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// ControlOutcome holds the side-channel outcome set by the pass or fail tools.
type ControlOutcome struct {
	Called  bool
	Pass    bool
	Message string
}

// MCPToolAdapter wraps an mcp.Tool and executes it via CallAgentTool,
// implementing ADK's tool.Tool and implicitly satisfying toolinternal interfaces.
type MCPToolAdapter struct {
	toolSchema mcp.Tool
	agent      *api.Agent
	mcpManager mcp.ConnectionManager
	servers    map[string]*api.LocalMCPServer
}

func (a *MCPToolAdapter) Name() string {
	return a.toolSchema.Name
}

func (a *MCPToolAdapter) Description() string {
	return a.toolSchema.Description
}

func (a *MCPToolAdapter) IsLongRunning() bool {
	return false
}

func (a *MCPToolAdapter) Declaration() *genai.FunctionDeclaration {
	decl := &genai.FunctionDeclaration{
		Name:        a.toolSchema.Name,
		Description: a.toolSchema.Description,
	}
	if a.toolSchema.InputSchema != nil {
		decl.ParametersJsonSchema = a.toolSchema.InputSchema
	}
	return decl
}

func (a *MCPToolAdapter) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	return packTool(req, a)
}

func (a *MCPToolAdapter) Run(ctx tool.Context, args any) (map[string]any, error) {
	var argMap map[string]interface{}
	if args != nil {
		if m, ok := args.(map[string]interface{}); ok {
			argMap = m
		} else {
			data, err := json.Marshal(args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal args: %w", err)
			}
			if err := json.Unmarshal(data, &argMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal args into map: %w", err)
			}
		}
	}
	if argMap == nil {
		argMap = make(map[string]interface{})
	}

	// Use the underlying engine context or context.Background() for the MCP call
	callCtx := context.Background()
	if ctx != nil {
		callCtx = ctx
	}

	res, err := CallAgentTool(callCtx, a.agent, a.mcpManager, a.servers, a.toolSchema.Name, argMap)
	if err != nil {
		return nil, fmt.Errorf("failed to call MCP tool %q: %w", a.toolSchema.Name, err)
	}

	if res.IsError {
		var details strings.Builder
		for _, c := range res.Content {
			if c.Type == "text" {
				details.WriteString(c.Text)
			}
		}
		errMsg := "Tool execution failed."
		if details.Len() > 0 {
			errMsg += " Details: " + details.String()
		}
		return nil, errors.New(errMsg)
	}

	var textResponse strings.Builder
	for _, c := range res.Content {
		if c.Type == "text" {
			textResponse.WriteString(c.Text)
		}
	}

	if textResponse.Len() == 0 {
		return nil, errors.New("no text content in tool response")
	}

	return map[string]any{
		"output": textResponse.String(),
	}, nil
}

// SyntheticControlTool implements ADK tool interfaces for synthetic tools "pass" and "fail".
type SyntheticControlTool struct {
	name        string
	description string
	inputSchema map[string]interface{}
	pass        bool
	outcome     *ControlOutcome
}

func (t *SyntheticControlTool) Name() string {
	return t.name
}

func (t *SyntheticControlTool) Description() string {
	return t.description
}

func (t *SyntheticControlTool) IsLongRunning() bool {
	return false
}

func (t *SyntheticControlTool) Declaration() *genai.FunctionDeclaration {
	decl := &genai.FunctionDeclaration{
		Name:        t.name,
		Description: t.description,
	}
	if t.inputSchema != nil {
		decl.ParametersJsonSchema = t.inputSchema
	}
	return decl
}

func (t *SyntheticControlTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	return packTool(req, t)
}

func (t *SyntheticControlTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	var msg string
	if args != nil {
		if m, ok := args.(map[string]interface{}); ok {
			if s, ok := m["message"].(string); ok {
				msg = s
			}
		} else if m, ok := args.(map[string]any); ok {
			if s, ok := m["message"].(string); ok {
				msg = s
			}
		} else {
			var payload struct {
				Message string `json:"message"`
			}
			data, _ := json.Marshal(args)
			_ = json.Unmarshal(data, &payload)
			msg = payload.Message
		}
	}

	if t.outcome != nil {
		t.outcome.Called = true
		t.outcome.Pass = t.pass
		t.outcome.Message = msg
	}

	if ctx != nil && ctx.Actions() != nil {
		if ctx.Actions().StateDelta == nil {
			ctx.Actions().StateDelta = make(map[string]any)
		}
		ctx.Actions().StateDelta["control_called"] = true
		ctx.Actions().StateDelta["control_pass"] = t.pass
		ctx.Actions().StateDelta["control_message"] = msg

		ctx.Actions().SkipSummarization = true
	}

	return map[string]any{
		"result": fmt.Sprintf("%s: %s", t.name, msg),
	}, nil
}

type internalTool interface {
	Name() string
	Declaration() *genai.FunctionDeclaration
}

func packTool(req *model.LLMRequest, t internalTool) error {
	if req.Tools == nil {
		req.Tools = make(map[string]any)
	}

	name := t.Name()

	if _, ok := req.Tools[name]; ok {
		return fmt.Errorf("duplicate tool: %q", name)
	}
	req.Tools[name] = t

	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}
	decl := t.Declaration()
	if decl == nil {
		return nil
	}

	var funcTool *genai.Tool
	for _, gt := range req.Config.Tools {
		if gt != nil && gt.FunctionDeclarations != nil {
			funcTool = gt
			break
		}
	}
	if funcTool == nil {
		req.Config.Tools = append(req.Config.Tools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{decl},
		})
	} else {
		funcTool.FunctionDeclarations = append(funcTool.FunctionDeclarations, decl)
	}
	return nil
}

// GetADKTools returns a list of ADK-compatible tools for the allowed MCP tools
// and synthetic control tools (pass/fail) for the given agent.
func GetADKTools(ctx context.Context, agent *api.Agent, mcpManager mcp.ConnectionManager, servers map[string]*api.LocalMCPServer, outcome *ControlOutcome) ([]tool.Tool, error) {
	var tools []tool.Tool

	if agent.Spec.Tools != nil {
		for _, provider := range agent.Spec.Tools {
			if provider.MCP != nil {
				mcpTools, err := getMCPTools(ctx, provider.MCP, mcpManager, servers)
				if err != nil {
					return nil, err
				}
				for _, t := range mcpTools {
					tools = append(tools, &MCPToolAdapter{
						toolSchema: t,
						agent:      agent,
						mcpManager: mcpManager,
						servers:    servers,
					})
				}
			}
		}
	}

	passSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Outcome message indicating success details",
			},
		},
		"required": []string{"message"},
	}

	tools = append(tools, &SyntheticControlTool{
		name:        "pass",
		description: "Indicates the agent successfully completed its task.",
		inputSchema: passSchema,
		pass:        true,
		outcome:     outcome,
	})

	failSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Outcome message indicating failure details",
			},
		},
		"required": []string{"message"},
	}

	tools = append(tools, &SyntheticControlTool{
		name:        "fail",
		description: "Indicates the agent encountered an unrecoverable error or could not make progress.",
		inputSchema: failSchema,
		pass:        false,
		outcome:     outcome,
	})

	return tools, nil
}
