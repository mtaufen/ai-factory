package history

import (
	"context"
	"fmt"
	"strings"
)

// LLMClient represents a generic interface for interacting with a Large Language Model.
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// LLMSummarizer implements Summarizer using an LLMClient.
type LLMSummarizer struct {
	client LLMClient
}

// NewLLMSummarizer creates a new LLMSummarizer.
func NewLLMSummarizer(client LLMClient) *LLMSummarizer {
	return &LLMSummarizer{
		client: client,
	}
}

// Summarize takes the current history and new messages, and returns a new
// condensed history containing a single summary message.
func (s *LLMSummarizer) Summarize(ctx context.Context, current History, newMessages History) (History, error) {
	var sb strings.Builder
	sb.WriteString("Please summarize the following execution history and latest results into a concise summary.\n")
	sb.WriteString("This summary will be used as the context for the next step in the execution loop.\n\n")

	if len(current) > 0 {
		sb.WriteString("--- Current Context Summary ---\n")
		for _, msg := range current {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
		}
		sb.WriteString("\n")
	}

	if len(newMessages) > 0 {
		sb.WriteString("--- Latest Execution Results ---\n")
		for _, msg := range newMessages {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
		}
		sb.WriteString("\n")
	}

	prompt := sb.String()

	summaryText, err := s.client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	return History{
		{
			Role:    "system",
			Content: summaryText,
		},
	}, nil
}
