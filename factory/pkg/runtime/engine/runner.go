package engine

import (
	"context"
	"fmt"

	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/history"
)

const MaxCallDepth = 1000

type StackFrame struct {
	LoopName       string
	StepName       string
	Args           map[string]string
	Steps          int
	NestedReturned bool
	CurrentHistory history.History
}

func (r *Runner) ExecuteLoop(ctx context.Context, loopName string, args map[string]string) (bool, string, history.History, error) {
	loop, ok := r.Loops[loopName]
	if !ok {
		return false, "", nil, fmt.Errorf("loop not found: %s", loopName)
	}

	stack := []*StackFrame{
		{
			LoopName:       loopName,
			StepName:       loop.Spec.Start,
			Args:           args,
			Steps:          0,
			CurrentHistory: nil,
		},
	}

	var lastPass bool
	var lastMessage string
	var lastHistory history.History

	for len(stack) > 0 {
		select {
		case <-ctx.Done():
			return false, "", nil, ctx.Err()
		default:
		}

		if len(stack) > MaxCallDepth {
			return false, "", nil, fmt.Errorf("max call depth %d exceeded", MaxCallDepth)
		}

		frame := stack[len(stack)-1]

		currentLoop, ok := r.Loops[frame.LoopName]
		if !ok {
			return false, "", nil, fmt.Errorf("loop not found: %s", frame.LoopName)
		}

		var step *api.Step
		for i := range currentLoop.Spec.Steps {
			if currentLoop.Spec.Steps[i].Name == frame.StepName {
				step = &currentLoop.Spec.Steps[i]
				break
			}
		}
		if step == nil {
			return false, "", nil, fmt.Errorf("step not found: %s", frame.StepName)
		}

		if !(step.Loop != nil && frame.NestedReturned) {
			r.GlobalSteps++
			frame.Steps++

			if r.Run != nil && r.Run.Spec.GlobalMaxSteps > 0 && r.GlobalSteps > r.Run.Spec.GlobalMaxSteps {
				return false, "", nil, fmt.Errorf("global max steps %d exceeded", r.Run.Spec.GlobalMaxSteps)
			}
			if currentLoop.Spec.MaxSteps > 0 && frame.Steps > currentLoop.Spec.MaxSteps {
				return false, "", nil, fmt.Errorf("loop max steps %d exceeded for loop %s", currentLoop.Spec.MaxSteps, frame.LoopName)
			}
		}

		var pass bool
		var message string
		var newMessages history.History
		var err error

		if step.Loop != nil {
			if !frame.NestedReturned {
				nestedArgs := InterpolateArgs(step.Loop.Args, frame.Args)
				nestedLoopName := step.Loop.Name
				nestedLoop, ok := r.Loops[nestedLoopName]
				if !ok {
					return false, "", nil, fmt.Errorf("loop not found: %s", nestedLoopName)
				}

				frame.NestedReturned = true

				stack = append(stack, &StackFrame{
					LoopName:       nestedLoopName,
					StepName:       nestedLoop.Spec.Start,
					Args:           nestedArgs,
					Steps:          0,
					CurrentHistory: nil,
				})
				continue
			} else {
				pass = lastPass
				message = lastMessage
				newMessages = lastHistory
				frame.NestedReturned = false
			}
		} else {
			if r.Executor == nil {
				return false, "", nil, fmt.Errorf("no executor provided for step %s", step.Name)
			}
			pass, message, newMessages, err = r.Executor.Execute(ctx, step, frame.Args, frame.CurrentHistory)
		}

		if err != nil {
			return false, "", nil, err
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

		switch nextAction.History {
		case api.HistoryNone, "":
			frame.CurrentHistory = nil
		case api.HistoryFull:
			frame.CurrentHistory = append(frame.CurrentHistory, newMessages...)
		case api.HistorySummary:
			if r.Summarizer != nil {
				frame.CurrentHistory, err = r.Summarizer.Summarize(ctx, frame.CurrentHistory, newMessages)
				if err != nil {
					return false, "", nil, fmt.Errorf("summarization failed: %w", err)
				}
			} else {
				return false, "", nil, fmt.Errorf("no summarizer provided for history: summary")
			}
		}

		switch nextAction.Next {
		case "return":
			lastHistory = frame.CurrentHistory
			stack = stack[:len(stack)-1]
		case "retry":
			// Keep frame.StepName the same
		default:
			if nextAction.Next == "" {
				return false, "", nil, fmt.Errorf("next step not specified in step %s", frame.StepName)
			}
			frame.StepName = nextAction.Next
		}
	}

	return lastPass, lastMessage, lastHistory, nil
}
