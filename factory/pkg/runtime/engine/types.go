package engine

import (
	"context"

	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

// Runner represents the Loop Execution Engine state machine.
// It tracks the Run state, available Loops, and global execution steps.
type Runner struct {
	Run         *api.Run
	Loops       map[string]*api.Loop
	GlobalSteps int
	Executor    StepExecutor
}

// StepExecutor defines the interface for executing a single step.
// It allows the Runner to execute steps independently of their implementation type
// (e.g., MCP, Loop, or Agent actions).
type StepExecutor interface {
	Execute(ctx context.Context, step *api.Step, args map[string]string) (pass bool, message string, err error)
}
