package engine

import (
	"context"
	"fmt"

	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

const MaxCallDepth = 1000

func (r *Runner) ExecuteLoop(ctx context.Context, loopName string, args map[string]string) (bool, string, error) {
	return r.executeLoopWithDepth(ctx, loopName, args, 0)
}

func (r *Runner) executeLoopWithDepth(ctx context.Context, loopName string, args map[string]string, depth int) (bool, string, error) {
	if depth > MaxCallDepth {
		return false, "", fmt.Errorf("max call depth %d exceeded", MaxCallDepth)
	}

	loop, ok := r.Loops[loopName]
	if !ok {
		return false, "", fmt.Errorf("loop not found: %s", loopName)
	}

	steps := make(map[string]*api.Step)
	for i := range loop.Spec.Steps {
		s := &loop.Spec.Steps[i]
		steps[s.Name] = s
	}

	currentStepName := loop.Spec.Start
	loopSteps := 0

	var lastPass bool
	var lastMessage string

	for {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}

		step, ok := steps[currentStepName]
		if !ok {
			return false, "", fmt.Errorf("step not found: %s", currentStepName)
		}

		r.GlobalSteps++
		loopSteps++

		if r.Run != nil && r.Run.Spec.GlobalMaxSteps > 0 && r.GlobalSteps > r.Run.Spec.GlobalMaxSteps {
			return false, "", fmt.Errorf("global max steps %d exceeded", r.Run.Spec.GlobalMaxSteps)
		}
		if loop.Spec.MaxSteps > 0 && loopSteps > loop.Spec.MaxSteps {
			return false, "", fmt.Errorf("loop max steps %d exceeded for loop %s", loop.Spec.MaxSteps, loopName)
		}

		var pass bool
		var message string
		var err error

		if step.Loop != nil {
			nestedArgs := InterpolateArgs(step.Loop.Args, args)
			pass, message, err = r.executeLoopWithDepth(ctx, step.Loop.Name, nestedArgs, depth+1)
		} else {
			if r.Executor == nil {
				return false, "", fmt.Errorf("no executor provided for step %s", step.Name)
			}
			pass, message, err = r.Executor.Execute(ctx, step, args)
		}

		if err != nil {
			return false, "", err
		}

		lastPass = pass
		lastMessage = message

		var nextAction api.NextAction
		if pass {
			nextAction = step.Pass
		} else {
			nextAction = step.Fail
		}

		if nextAction.Message != "" {
			lastMessage = nextAction.Message
		}

		switch nextAction.Next {
		case "return":
			return lastPass, lastMessage, nil
		case "retry":
			// Keep currentStepName the same
		default:
			if nextAction.Next == "" {
				return false, "", fmt.Errorf("next step not specified in step %s", currentStepName)
			}
			currentStepName = nextAction.Next
		}
	}
}
