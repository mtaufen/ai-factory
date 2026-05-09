package loop

import (
	"context"
	"fmt"
	"os"

	"github.com/ai-on-gke/ai-factory/factory/pkg/mcp"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/engine"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/history"
	"github.com/spf13/cobra"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

var configPath string


type noopExecutor struct{}

func (e *noopExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string, currentHistory history.History) (bool, string, history.History, error) {
	return true, "noop", nil, nil
}

var Cmd = &cobra.Command{
	Use:   "loop",
	Short: "Execute a factory runtime loop",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("GEMINI_API_KEY environment variable is required")
		}

		if configPath == "" {
			return fmt.Errorf("--config flag is required")
		}

		file, err := os.Open(configPath)
		if err != nil {
			return fmt.Errorf("failed to open config file %s: %w", configPath, err)
		}
		defer file.Close()

		unstructuredObjs, err := api.Parse(file)
		if err != nil {
			return fmt.Errorf("failed to parse manifests from %s: %w", configPath, err)
		}

		runs := make(map[string]*api.Run)
		loops := make(map[string]*api.Loop)
		agents := make(map[string]*api.Agent)
		servers := make(map[string]*api.LocalMCPServer)

		for _, u := range unstructuredObjs {
			switch u.GetKind() {
			case "Run":
				var run api.Run
				if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &run); err != nil {
					return fmt.Errorf("failed to convert Run: %w", err)
				}
				runs[run.Name] = &run
			case "Loop":
				var loop api.Loop
				if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &loop); err != nil {
					return fmt.Errorf("failed to convert Loop: %w", err)
				}
				loops[loop.Name] = &loop
			case "Agent":
				var agent api.Agent
				if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &agent); err != nil {
					return fmt.Errorf("failed to convert Agent: %w", err)
				}
				agents[agent.Name] = &agent
			case "LocalMCPServer":
				var server api.LocalMCPServer
				if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &server); err != nil {
					return fmt.Errorf("failed to convert LocalMCPServer: %w", err)
				}
				servers[server.Name] = &server
			default:
				// ignore other kinds
			}
		}

		var run *api.Run
		for _, r := range runs {
			run = r
			break
		}

		if run == nil {
			return fmt.Errorf("no Run resource found in manifests")
		}

		mcpManager := mcp.NewConnectionManager()

		ctx := cmd.Context()
		clientConfig := &genai.ClientConfig{APIKey: apiKey}
		geminiModel, err := gemini.NewModel(ctx, "gemini-3.1-pro-preview", clientConfig)
		if err != nil {
			return fmt.Errorf("failed to initialize gemini model: %w", err)
		}

		llmClient := history.NewADKModelAdapter(geminiModel)
		summarizer := history.NewLLMSummarizer(llmClient)

		agentExecutor := engine.NewAgentExecutor(agents, mcpManager, servers, geminiModel)
		mcpExecutor := engine.NewMCPStepExecutor(mcpManager, servers)
		stdExecutor := &noopExecutor{}

		runner := &engine.Runner{
			Run:           run,
			Loops:         loops,
			Executor:      stdExecutor,
			MCPExecutor:   mcpExecutor,
			AgentExecutor: agentExecutor,
			Summarizer:    summarizer,
		}

		pass, message, _, err := runner.ExecuteLoop(ctx, run.Spec.Start, nil)
		if err != nil {
			return fmt.Errorf("loop execution failed: %w", err)
		}

		if !pass {
			return fmt.Errorf("Loop failed: %s", message)
		}

		fmt.Printf("Loop completed successfully: %s\n", message)
		return nil
	},
}

func init() {
	Cmd.Flags().StringVar(&configPath, "config", "", "Path to YAML manifests defining the Run, Loop, Agent, and LocalMCPServer resources")
	Cmd.MarkFlagRequired("config")
}
