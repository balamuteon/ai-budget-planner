package ai

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client interface {
	Chat(ctx context.Context, messages []Message) (string, []byte, error)
}

func resolveMaxTokens(value int) int {
	if value > 0 {
		return value
	}

	return defaultMaxTokens
}
