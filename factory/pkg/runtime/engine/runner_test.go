package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

type stubExecutor struct {
	results map[string]struct {
		pass    bool
		message string
		err     error
	}
	execCount map[string]int
}

func (e *stubExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string) (bool, string, error) {
	if e.execCount == nil {
		e.execCount = make(map[string]int)
	}
	e.execCount[step.Name]++

	if e.results != nil {
		res, ok := e.results[step.Name]
		if ok {
			return res.pass, res.message, res.err
		}
	}
	return true, "stub success", nil
}

func TestRunner_ExecuteLoop_Transitions(t *testing.T) {
	run := &api.Run{}
	loop := &api.Loop{
		Spec: api.LoopSpec{
			Start: "step1",
			Steps: []api.Step{
				{
					Name: "step1",
					Pass: api.NextAction{Next: "step2"},
				},
				{
					Name: "step2",
					Fail: api.NextAction{Next: "step3"},
				},
				{
					Name: "step3",
					Pass: api.NextAction{Next: "return", Message: "final return"},
				},
			},
		},
	}

	exec := &stubExecutor{
		results: map[string]struct {
			pass    bool
			message string
			err     error
		}{
			"step1": {pass: true, message: "step1 ok"},
			"step2": {pass: false, message: "step2 failed"},
			"step3": {pass: true, message: "step3 ok"},
		},
	}

	runner := &Runner{
		Run:   run,
		Loops: map[string]*api.Loop{"main": loop},
		Executor: exec,
	}

	pass, msg, err := runner.ExecuteLoop(context.Background(), "main", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Errorf("expected pass=true")
	}
	if msg != "final return" {
		t.Errorf("expected message 'final return', got %q", msg)
	}
	if runner.GlobalSteps != 3 {
		t.Errorf("expected 3 steps executed, got %d", runner.GlobalSteps)
	}
	for _, s := range []string{"step1", "step2", "step3"} {
		if exec.execCount[s] != 1 {
			t.Errorf("expected step %s to run 1 time, got %d", s, exec.execCount[s])
		}
	}
}

func TestRunner_ExecuteLoop_Retry(t *testing.T) {
	run := &api.Run{}
	loop := &api.Loop{
		Spec: api.LoopSpec{
			Start: "step1",
			Steps: []api.Step{
				{
					Name: "step1",
					Pass: api.NextAction{Next: "return"},
					Fail: api.NextAction{Next: "retry"},
				},
			},
		},
	}

	execCount := 0

	runner := &Runner{
		Run:   run,
		Loops: map[string]*api.Loop{"main": loop},
		Executor: &customExecutor{count: &execCount},
	}

	pass, _, err := runner.ExecuteLoop(context.Background(), "main", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Errorf("expected pass=true")
	}
	if execCount != 3 {
		t.Errorf("expected 3 executions due to retry, got %d", execCount)
	}
	if runner.GlobalSteps != 3 {
		t.Errorf("expected 3 global steps, got %d", runner.GlobalSteps)
	}
}

type customExecutor struct {
	count *int
}

func (e *customExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string) (bool, string, error) {
	*e.count++
	if *e.count < 3 {
		return false, "failed", nil
	}
	return true, "success", nil
}

func TestRunner_ExecuteLoop_ContextCancellation(t *testing.T) {
	run := &api.Run{}
	loop := &api.Loop{
		Spec: api.LoopSpec{
			Start: "step1",
			Steps: []api.Step{
				{
					Name: "step1",
					Pass: api.NextAction{Next: "retry"}, // infinite loop
				},
			},
		},
	}

	runner := &Runner{
		Run:   run,
		Loops: map[string]*api.Loop{"main": loop},
		Executor: &stubExecutor{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, _, err := runner.ExecuteLoop(ctx, "main", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRunner_ExecuteLoop_GlobalMaxSteps(t *testing.T) {
	run := &api.Run{
		Spec: api.RunSpec{GlobalMaxSteps: 5},
	}
	loop := &api.Loop{
		Spec: api.LoopSpec{
			Start: "step1",
			Steps: []api.Step{
				{
					Name: "step1",
					Pass: api.NextAction{Next: "retry"}, // infinite loop
				},
			},
		},
	}

	runner := &Runner{
		Run:   run,
		Loops: map[string]*api.Loop{"main": loop},
		Executor: &stubExecutor{},
	}

	_, _, err := runner.ExecuteLoop(context.Background(), "main", nil)
	if err == nil || err.Error() != "global max steps 5 exceeded" {
		t.Errorf("expected global max steps error, got %v", err)
	}
	if runner.GlobalSteps != 6 {
		t.Errorf("expected 6 global steps before failure, got %d", runner.GlobalSteps)
	}
}

func TestRunner_ExecuteLoop_LoopMaxSteps(t *testing.T) {
	run := &api.Run{}
	loop := &api.Loop{
		Spec: api.LoopSpec{
			Start: "step1",
			MaxSteps: 3,
			Steps: []api.Step{
				{
					Name: "step1",
					Pass: api.NextAction{Next: "retry"}, // infinite loop
				},
			},
		},
	}

	runner := &Runner{
		Run:   run,
		Loops: map[string]*api.Loop{"main": loop},
		Executor: &stubExecutor{},
	}

	_, _, err := runner.ExecuteLoop(context.Background(), "main", nil)
	if err == nil || err.Error() != "loop max steps 3 exceeded for loop main" {
		t.Errorf("expected loop max steps error, got %v", err)
	}
}

func TestRunner_ExecuteLoop_NestedLoop(t *testing.T) {
	run := &api.Run{}
	mainLoop := &api.Loop{
		Spec: api.LoopSpec{
			Start: "step1",
			Steps: []api.Step{
				{
					Name: "step1",
					Loop: &api.LoopAction{
						Name: "sub",
						Args: []api.Argument{{Name: "ARG1", Value: "$(MY_ARG)"}},
					},
					Pass: api.NextAction{Next: "return"},
				},
			},
		},
	}
	subLoop := &api.Loop{
		Spec: api.LoopSpec{
			Start: "sub1",
			Steps: []api.Step{
				{
					Name: "sub1",
					Pass: api.NextAction{Next: "return"},
				},
			},
		},
	}

	var subArgs map[string]string
	exec := &argCaptureExecutor{
		capture: &subArgs,
	}

	runner := &Runner{
		Run:   run,
		Loops: map[string]*api.Loop{"main": mainLoop, "sub": subLoop},
		Executor: exec,
	}

	pass, _, err := runner.ExecuteLoop(context.Background(), "main", map[string]string{"MY_ARG": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Errorf("expected pass=true")
	}
	if subArgs["ARG1"] != "hello" {
		t.Errorf("expected interpolated arg 'hello', got %q", subArgs["ARG1"])
	}
}

type argCaptureExecutor struct {
	capture *map[string]string
}

func (e *argCaptureExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string) (bool, string, error) {
	*e.capture = args
	return true, "ok", nil
}

func TestRunner_ExecuteLoop_CyclicDependencyPrevention(t *testing.T) {
	run := &api.Run{}
	loop := &api.Loop{
		Spec: api.LoopSpec{
			Start: "step1",
			Steps: []api.Step{
				{
					Name: "step1",
					Loop: &api.LoopAction{
						Name: "main", // self referential loop
					},
					Pass: api.NextAction{Next: "return"},
				},
			},
		},
	}

	runner := &Runner{
		Run:   run,
		Loops: map[string]*api.Loop{"main": loop},
		Executor: &stubExecutor{},
	}

	_, _, err := runner.ExecuteLoop(context.Background(), "main", nil)
	if err == nil || err.Error() != "max call depth 1000 exceeded" {
		t.Errorf("expected max call depth error, got %v", err)
	}
}

