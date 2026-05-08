package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/history"
)

type mockSummarizer struct {
	count int
}

func (m *mockSummarizer) Summarize(ctx context.Context, current history.History, newMessages history.History) (history.History, error) {
	m.count++
	return history.History{{Role: "system", Content: "mocked summary"}}, nil
}

type stubExecutor struct {
	results map[string]struct {
		pass    bool
		message string
		err     error
	}
	execCount map[string]int
}

func (e *stubExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string, currentHistory history.History) (bool, string, history.History, error) {
	if e.execCount == nil {
		e.execCount = make(map[string]int)
	}
	e.execCount[step.Name]++

	if e.results != nil {
		res, ok := e.results[step.Name]
		if ok {
			return res.pass, res.message, history.History{{Role: "system", Content: res.message}}, res.err
		}
	}
	return true, "stub success", history.History{{Role: "system", Content: "stub success"}}, nil
}

type customExecutor struct {
	count int
}

func (e *customExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string, currentHistory history.History) (bool, string, history.History, error) {
	e.count++
	if e.count < 3 {
		return false, "failed", history.History{{Role: "system", Content: "failed"}}, nil
	}
	return true, "success", history.History{{Role: "system", Content: "success"}}, nil
}

type argCaptureExecutor struct {
	capture map[string]string
}

func (e *argCaptureExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string, currentHistory history.History) (bool, string, history.History, error) {
	e.capture = args
	return true, "ok", nil, nil
}

type deleteLoopExecutor struct {
	runner *Runner
	loopToDelete string
}

func (e *deleteLoopExecutor) Execute(ctx context.Context, step *api.Step, args map[string]string, currentHistory history.History) (bool, string, history.History, error) {
	delete(e.runner.Loops, e.loopToDelete)
	return true, "ok", nil, nil
}

func TestRunner_ExecuteLoop(t *testing.T) {
	tests := []struct {
		name        string
		run         *api.Run
		loops       map[string]*api.Loop
		executor    StepExecutor
		startLoop   string
		startArgs   map[string]string
		cancelCtx   bool
		wantErr     string
		wantPass    bool
		wantMsg     string
		wantGlobal  int
		summarizer  history.Summarizer
		checkHistory func(t *testing.T, h history.History)
		checkExtra  func(t *testing.T, exec StepExecutor)
	}{
		{
			name: "HistoryNone",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "step2", History: api.HistoryFull}},
							{Name: "step2", Pass: api.NextAction{Next: "step3", History: api.HistoryNone}},
							{Name: "step3", Pass: api.NextAction{Next: "return", History: api.HistoryFull}},
						},
					},
				},
			},
			executor: &stubExecutor{
				results: map[string]struct {
					pass    bool
					message string
					err     error
				}{
					"step1": {pass: true, message: "msg1"},
					"step2": {pass: true, message: "msg2"},
					"step3": {pass: true, message: "msg3"},
				},
			},
			startLoop: "main",
			wantPass:  true,
			checkHistory: func(t *testing.T, h history.History) {
				if len(h) != 1 || h[0].Content != "msg3" {
					t.Errorf("expected history to contain only msg3 due to history: none, got %v", h)
				}
			},
		},
		{
			name: "HistoryFull",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "step2", History: api.HistoryFull}},
							{Name: "step2", Pass: api.NextAction{Next: "return", History: api.HistoryFull}},
						},
					},
				},
			},
			executor: &stubExecutor{
				results: map[string]struct {
					pass    bool
					message string
					err     error
				}{
					"step1": {pass: true, message: "msg1"},
					"step2": {pass: true, message: "msg2"},
				},
			},
			startLoop: "main",
			wantPass:  true,
			checkHistory: func(t *testing.T, h history.History) {
				if len(h) != 2 || h[0].Content != "msg1" || h[1].Content != "msg2" {
					t.Errorf("expected history to contain msg1 and msg2, got %v", h)
				}
			},
		},
		{
			name: "HistorySummary",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "step2", History: api.HistoryFull}},
							{Name: "step2", Pass: api.NextAction{Next: "return", History: api.HistorySummary}},
						},
					},
				},
			},
			executor: &stubExecutor{
				results: map[string]struct {
					pass    bool
					message string
					err     error
				}{
					"step1": {pass: true, message: "msg1"},
					"step2": {pass: true, message: "msg2"},
				},
			},
			summarizer: &mockSummarizer{},
			startLoop:  "main",
			wantPass:   true,
			checkHistory: func(t *testing.T, h history.History) {
				if len(h) != 1 || h[0].Content != "mocked summary" {
					t.Errorf("expected history to contain mocked summary, got %v", h)
				}
			},
		},
		{
			name: "Transitions",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "step2"}},
							{Name: "step2", Fail: api.NextAction{Next: "step3"}},
							{Name: "step3", Pass: api.NextAction{Next: "return", Message: "final return"}},
						},
					},
				},
			},
			executor: &stubExecutor{
				results: map[string]struct {
					pass    bool
					message string
					err     error
				}{
					"step1": {pass: true, message: "step1 ok"},
					"step2": {pass: false, message: "step2 failed"},
					"step3": {pass: true, message: "step3 ok"},
				},
			},
			startLoop:  "main",
			wantPass:   true,
			wantMsg:    "final return",
			wantGlobal: 3,
			checkExtra: func(t *testing.T, exec StepExecutor) {
				stub := exec.(*stubExecutor)
				for _, s := range []string{"step1", "step2", "step3"} {
					if stub.execCount[s] != 1 {
						t.Errorf("expected step %s to run 1 time, got %d", s, stub.execCount[s])
					}
				}
			},
		},
		{
			name: "Retry",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "return"}, Fail: api.NextAction{Next: "retry"}},
						},
					},
				},
			},
			executor:   &customExecutor{},
			startLoop:  "main",
			wantPass:   true,
			wantGlobal: 3,
			checkExtra: func(t *testing.T, exec StepExecutor) {
				custom := exec.(*customExecutor)
				if custom.count != 3 {
					t.Errorf("expected 3 executions due to retry, got %d", custom.count)
				}
			},
		},
		{
			name: "ContextCancellation",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "retry"}},
						},
					},
				},
			},
			executor:  &stubExecutor{},
			startLoop: "main",
			cancelCtx: true,
			wantErr:   "canceled", // context.Canceled error contains "canceled"
		},
		{
			name: "GlobalMaxSteps",
			run:  &api.Run{Spec: api.RunSpec{GlobalMaxSteps: 5}},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "retry"}},
						},
					},
				},
			},
			executor:   &stubExecutor{},
			startLoop:  "main",
			wantErr:    "global max steps 5 exceeded",
			wantGlobal: 6,
		},
		{
			name: "LoopMaxSteps",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start:    "step1",
						MaxSteps: 3,
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "retry"}},
						},
					},
				},
			},
			executor:  &stubExecutor{},
			startLoop: "main",
			wantErr:   "loop max steps 3 exceeded for loop main",
		},
		{
			name: "NestedLoop",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{
								Name: "step1",
								Loop: &api.LoopAction{Name: "sub", Args: []api.Argument{{Name: "ARG1", Value: "$(MY_ARG)"}}},
								Pass: api.NextAction{Next: "return"},
							},
						},
					},
				},
				"sub": {
					Spec: api.LoopSpec{
						Start: "sub1",
						Steps: []api.Step{
							{Name: "sub1", Pass: api.NextAction{Next: "return"}},
						},
					},
				},
			},
			executor:  &argCaptureExecutor{},
			startLoop: "main",
			startArgs: map[string]string{"MY_ARG": "hello"},
			wantPass:  true,
			checkExtra: func(t *testing.T, exec StepExecutor) {
				capture := exec.(*argCaptureExecutor)
				if capture.capture["ARG1"] != "hello" {
					t.Errorf("expected interpolated arg 'hello', got %q", capture.capture["ARG1"])
				}
			},
		},
		{
			name: "CyclicDependencyPrevention",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{
								Name: "step1",
								Loop: &api.LoopAction{Name: "main"},
								Pass: api.NextAction{Next: "return"},
							},
						},
					},
				},
			},
			executor:  &stubExecutor{},
			startLoop: "main",
			wantErr:   "max call depth 1000 exceeded",
		},
		{
			name: "MidExecutionLoopDeletion",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: "step2"}},
							{Name: "step2", Pass: api.NextAction{Next: "return"}},
						},
					},
				},
			},
			startLoop: "main",
			wantErr:   "loop not found: main",
			// We set executor in checkExtra before starting the runner, or we inject it into the setup.
			// Actually, we can't inject runner into executor cleanly here without modifying the test loop structure.
			// Let's just create a custom executor implementation that uses a closure inside the test loop if needed.
		},
		{
			name: "EntryLoopNotFound",
			run:  &api.Run{},
			loops: map[string]*api.Loop{},
			executor:  &stubExecutor{},
			startLoop: "missing",
			wantErr:   "loop not found: missing",
		},
		{
			name: "StepNotFound",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "missing_step",
						Steps: []api.Step{},
					},
				},
			},
			executor:  &stubExecutor{},
			startLoop: "main",
			wantErr:   "step not found: missing_step",
		},
		{
			name: "NestedLoopNotFound",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{
								Name: "step1",
								Loop: &api.LoopAction{Name: "missing_nested"},
							},
						},
					},
				},
			},
			executor:  &stubExecutor{},
			startLoop: "main",
			wantErr:   "loop not found: missing_nested",
		},
		{
			name: "NoExecutor",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1"},
						},
					},
				},
			},
			executor:  nil,
			startLoop: "main",
			wantErr:   "no executor provided for step step1",
		},
		{
			name: "ExecutorError",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1"},
						},
					},
				},
			},
			executor: &stubExecutor{
				results: map[string]struct{
					pass bool
					message string
					err error
				}{
					"step1": {err: errors.New("hard executor error")},
				},
			},
			startLoop: "main",
			wantErr:   "hard executor error",
		},
		{
			name: "NextStepNotSpecified",
			run:  &api.Run{},
			loops: map[string]*api.Loop{
				"main": {
					Spec: api.LoopSpec{
						Start: "step1",
						Steps: []api.Step{
							{Name: "step1", Pass: api.NextAction{Next: ""}},
						},
					},
				},
			},
			executor:  &stubExecutor{},
			startLoop: "main",
			wantErr:   "next step not specified in step step1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &Runner{
				Run:        tt.run,
				Loops:      tt.loops,
				Executor:   tt.executor,
				Summarizer: tt.summarizer,
			}

			if tt.name == "MidExecutionLoopDeletion" {
				runner.Executor = &deleteLoopExecutor{
					runner: runner,
					loopToDelete: "main",
				}
			}

			ctx := context.Background()
			if tt.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // cancel immediately
			}

			pass, msg, finalHistory, err := runner.ExecuteLoop(ctx, tt.startLoop, tt.startArgs)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pass != tt.wantPass {
				t.Errorf("expected pass=%v, got %v", tt.wantPass, pass)
			}

			if tt.wantMsg != "" && msg != tt.wantMsg {
				t.Errorf("expected message %q, got %q", tt.wantMsg, msg)
			}

			if tt.wantGlobal > 0 && runner.GlobalSteps != tt.wantGlobal {
				t.Errorf("expected %d global steps, got %d", tt.wantGlobal, runner.GlobalSteps)
			}

			if tt.checkHistory != nil {
				tt.checkHistory(t, finalHistory)
			}

			if tt.checkExtra != nil {
				tt.checkExtra(t, tt.executor)
			}
		})
	}
}
