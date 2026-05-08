package history

import (
	"context"
)

// Message represents a single message in the execution history.
// It can represent an agent turn, tool call, or a summary of previous context.
type Message struct {
	// Role could be "user", "assistant", "system", "tool", etc.
	Role    string `json:"role"`
	Content string `json:"content"`
}

// History represents the context of execution up to a certain point.
type History []Message

// Summarizer defines the interface for compressing history contexts.
type Summarizer interface {
	// Summarize takes the current history and new messages, and returns a new
	// condensed history (typically containing a single summary message).
	Summarize(ctx context.Context, current History, newMessages History) (History, error)
}
