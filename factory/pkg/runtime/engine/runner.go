package engine

import (
	"context"
	"fmt"

	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

const MaxCallDepth = 1000

type StackFrame struct {
	LoopName       string
	StepName       string
	Args           map[string]string
	Steps          int
	NestedReturned bool
}

func (r *Runner) ExecuteLoop(ctx context.Context, loopName string, args map[string]string) (bool, string, error) {
	loop, ok := r.Loops[loopName]
	if !ok {
		return false, "", fmt.Errorf("loop not found: %s", loopName)
	}

	stack := []*StackFrame{
		{
			LoopName: loopName,
			StepName: loop.Spec.Start,
			Args:     args,
			Steps:    0,
		},
	}

	var lastPass bool
	var lastMessage string

	for len(stack) > 0 {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}

		if len(stack) > MaxCallDepth {
			return false, "", fmt.Errorf("max call depth %d exceeded", MaxCallDepth)
		}

		frame := stack[len(stack)-1]

		currentLoop, ok := r.Loops[frame.LoopName]
		if !ok {
			return false, "", fmt.Errorf("loop not found: %s", frame.LoopName)
		}

		var step *api.Step
		for i := range currentLoop.Spec.Steps {
			if currentLoop.Spec.Steps[i].Name == frame.StepName {
				step = &currentLoop.Spec.Steps[i]
				break
			}
		}
		if step == nil {
			return false, "", fmt.Errorf("step not found: %s", frame.StepName)
		}

		if !(step.Loop != nil && frame.NestedReturned) {
			r.GlobalSteps++
			frame.Steps++

			if r.Run != nil && r.Run.Spec.GlobalMaxSteps > 0 && r.GlobalSteps > r.Run.Spec.GlobalMaxSteps {
				return false, "", fmt.Errorf("global max steps %d exceeded", r.Run.Spec.GlobalMaxSteps)
			}
			if currentLoop.Spec.MaxSteps > 0 && frame.Steps > currentLoop.Spec.MaxSteps {
				return false, "", fmt.Errorf("loop max steps %d exceeded for loop %s", currentLoop.Spec.MaxSteps, frame.LoopName)
			}
		}

		var pass bool
		var message string
		var err error

		if step.Loop != nil {
			if !frame.NestedReturned {
				nestedArgs := InterpolateArgs(step.Loop.Args, frame.Args)
				nestedLoopName := step.Loop.Name
				nestedLoop, ok := r.Loops[nestedLoopName]
				if !ok {
					return false, "", fmt.Errorf("loop not found: %s", nestedLoopName)
				}

				frame.NestedReturned = true

				stack = append(stack, &StackFrame{
					LoopName: nestedLoopName,
					StepName: nestedLoop.Spec.Start,
					Args:     nestedArgs,
					Steps:    0,
				})
				continue
			} else {
				pass = lastPass
				message = lastMessage
				frame.NestedReturned = false
			}
		} else {
			if r.Executor == nil {
				return false, "", fmt.Errorf("no executor provided for step %s", step.Name)
			}
			pass, message, err = r.Executor.Execute(ctx, step, frame.Args)
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
			stack = stack[:len(stack)-1]
		case "retry":
			// Keep frame.StepName the same
		default:
			if nextAction.Next == "" {
				return false, "", fmt.Errorf("next step not specified in step %s", frame.StepName)
			}
			frame.StepName = nextAction.Next
		}
	}

	return lastPass, lastMessage, nil
}
