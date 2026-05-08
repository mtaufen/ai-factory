package history

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// MockLLMClient implements LLMClient for testing.
type MockLLMClient struct {
	GenerateFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *MockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, prompt)
	}
	return "", nil
}

func TestLLMSummarizer_Summarize(t *testing.T) {
	tests := []struct {
		name         string
		current      History
		newMessages  History
		mockResponse string
		mockError    error
		expectedLen  int
		expectedRole string
		expectedText string
		expectErr    bool
	}{
		{
			name: "successful summarization",
			current: History{
				{Role: "system", Content: "previous summary"},
			},
			newMessages: History{
				{Role: "assistant", Content: "did a thing"},
			},
			mockResponse: "new consolidated summary",
			expectedLen:  1,
			expectedRole: "system",
			expectedText: "new consolidated summary",
			expectErr:    false,
		},
		{
			name: "llm error",
			current: History{
				{Role: "system", Content: "previous summary"},
			},
			newMessages: History{
				{Role: "assistant", Content: "did a thing"},
			},
			mockError: errors.New("llm failure"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &MockLLMClient{
				GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			summarizer := NewLLMSummarizer(mockLLM)
			result, err := summarizer.Summarize(context.Background(), tt.current, tt.newMessages)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != tt.expectedLen {
				t.Errorf("expected history length %d, got %d", tt.expectedLen, len(result))
			}

			if len(result) > 0 {
				if result[0].Role != tt.expectedRole {
					t.Errorf("expected role %s, got %s", tt.expectedRole, result[0].Role)
				}
				if result[0].Content != tt.expectedText {
					t.Errorf("expected content %s, got %s", tt.expectedText, result[0].Content)
				}
			}
		})
	}
}

func TestLLMSummarizer_PromptFormatting(t *testing.T) {
	var capturedPrompt string
	mockLLM := &MockLLMClient{
		GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
			capturedPrompt = prompt
			return "ok", nil
		},
	}

	summarizer := NewLLMSummarizer(mockLLM)
	current := History{{Role: "user", Content: "hello"}}
	newMsgs := History{{Role: "assistant", Content: "world"}}

	_, err := summarizer.Summarize(context.Background(), current, newMsgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPrompt == "" {
		t.Errorf("prompt was empty")
	}

	if !strings.Contains(capturedPrompt, "hello") {
		t.Errorf("expected prompt to contain 'hello', got:\n%s", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "world") {
		t.Errorf("expected prompt to contain 'world', got:\n%s", capturedPrompt)
	}
}
