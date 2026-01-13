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

// GeminiClient calls the Google Generative Language API (Gemini).
type GeminiClient struct {
	apiKey     string
	baseURL    string
	model      string
	maxTokens  int
	httpClient *http.Client
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  *geminiConfig   `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewGeminiClient создает клиент Gemini с заданными параметрами.
func NewGeminiClient(apiKey, baseURL, model string, timeout time.Duration, maxTokens int) *GeminiClient {
	trimmedURL := strings.TrimRight(baseURL, "/")
	return &GeminiClient{
		apiKey:    apiKey,
		baseURL:   trimmedURL,
		model:     model,
		maxTokens: maxTokens,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Chat отправляет сообщения в Gemini и возвращает текст ответа и сырой ответ API.
func (c *GeminiClient) Chat(ctx context.Context, messages []Message) (string, []byte, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", nil, errors.New("gemini api key is missing")
	}

	systemParts := make([]geminiPart, 0)
	contents := make([]geminiContent, 0)

	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}

		switch role {
		case "system":
			systemParts = append(systemParts, geminiPart{Text: text})
		case "assistant", "model":
			contents = append(contents, geminiContent{Role: "model", Parts: []geminiPart{{Text: text}}})
		default:
			contents = append(contents, geminiContent{Role: "user", Parts: []geminiPart{{Text: text}}})
		}
	}

	if len(contents) == 0 {
		return "", nil, errors.New("gemini request has no user content")
	}

	request := geminiRequest{
		Contents: contents,
		GenerationConfig: &geminiConfig{
			Temperature:      0.2,
			MaxOutputTokens:  resolveMaxTokens(c.maxTokens),
			ResponseMimeType: "application/json",
		},
	}

	if len(systemParts) > 0 {
		request.SystemInstruction = &geminiContent{Role: "system", Parts: systemParts}
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return "", nil, err
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", nil, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var apiErr geminiResponse
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != nil {
			return "", body, fmt.Errorf("gemini api error: %s", apiErr.Error.Message)
		}
		return "", body, fmt.Errorf("gemini api error: %s", strings.TrimSpace(string(body)))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", body, err
	}

	if len(parsed.Candidates) == 0 {
		return "", body, errors.New("gemini response missing candidates")
	}

	parts := parsed.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return "", body, errors.New("gemini response missing content")
	}

	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(part.Text)
	}

	return builder.String(), body, nil
}
