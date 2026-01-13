package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultMaxTokens = 4096

// GroqClient calls the Groq OpenAI-compatible chat completions API.
type GroqClient struct {
	apiKey     string
	baseURL    string
	model      string
	maxTokens  int
	httpClient *http.Client
}

type groqChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type groqChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewGroqClient создает клиент Groq с заданными параметрами.
func NewGroqClient(apiKey, baseURL, model string, timeout time.Duration, maxTokens int) *GroqClient {
	trimmedURL := strings.TrimRight(baseURL, "/")
	return &GroqClient{
		apiKey:    apiKey,
		baseURL:   trimmedURL,
		model:     model,
		maxTokens: maxTokens,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Chat отправляет сообщения в Groq и возвращает текст ответа и сырой ответ API.
func (c *GroqClient) Chat(ctx context.Context, messages []Message) (string, []byte, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", nil, errors.New("groq api key is missing")
	}

	reqBody := groqChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.2,
		MaxTokens:   resolveMaxTokens(c.maxTokens),
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	endpoint := fmt.Sprintf("%s/chat/completions", c.baseURL)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", nil, err
	}

	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", nil, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var apiErr groqChatResponse
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != nil {
			return "", body, fmt.Errorf("groq api error: %s", apiErr.Error.Message)
		}
		return "", body, fmt.Errorf("groq api error: %s", strings.TrimSpace(string(body)))
	}

	var parsed groqChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", body, err
	}

	if len(parsed.Choices) == 0 {
		return "", body, errors.New("groq response missing choices")
	}

	return parsed.Choices[0].Message.Content, body, nil
}
