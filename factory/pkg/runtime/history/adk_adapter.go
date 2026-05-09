package history

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// ADKModelAdapter implements LLMClient by wrapping an ADK model.LLM.
type ADKModelAdapter struct {
	llm model.LLM
}

// NewADKModelAdapter creates a new ADKModelAdapter.
func NewADKModelAdapter(llm model.LLM) *ADKModelAdapter {
	return &ADKModelAdapter{
		llm: llm,
	}
}

// Generate implements the LLMClient interface.
func (a *ADKModelAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	req := &model.LLMRequest{
		Contents: genai.Text(prompt),
	}

	var sb strings.Builder
	for resp, err := range a.llm.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", fmt.Errorf("failed to generate response using ADK model: %w", err)
		}
		if resp != nil && resp.Content != nil {
			for _, part := range resp.Content.Parts {
				if part.Text != "" {
					sb.WriteString(part.Text)
				}
			}
		}
	}

	return sb.String(), nil
}
