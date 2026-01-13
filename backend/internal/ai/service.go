package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	mandatoryType  = "mandatory"
	optionalType   = "optional"
	priorityRed    = "red"
	priorityYellow = "yellow"
	priorityGreen  = "green"
	noteTypeAI     = "ai"
	noteTypeUser   = "user"
)

type Service struct {
	client Client
}

// NewService создает сервис работы с AI-клиентом.
func NewService(client Client) *Service {
	return &Service{client: client}
}

// GeneratePlan запрашивает у AI план бюджета и валидирует ответ.
func (s *Service) GeneratePlan(ctx context.Context, input GeneratePlanInput) (PlanResponse, string, []byte, error) {
	prompt, err := buildGeneratePlanPrompt(input)
	if err != nil {
		return PlanResponse{}, "", nil, err
	}

	messages := []Message{
		{Role: "system", Content: "You are a budgeting assistant. Respond with JSON only, without extra text."},
		{Role: "user", Content: prompt},
	}

	content, raw, err := s.client.Chat(ctx, messages)
	if err != nil {
		return PlanResponse{}, prompt, raw, err
	}

	var response PlanResponse
	if err := parseJSON(content, &response); err != nil {
		return PlanResponse{}, prompt, raw, err
	}

	normalizePlanResponse(&response)
	if err := validatePlanResponse(response, input.BudgetCents); err != nil {
		return PlanResponse{}, prompt, raw, err
	}

	return response, prompt, raw, nil
}

// AnalyzeSpending запрашивает у AI рекомендации по расходам.
func (s *Service) AnalyzeSpending(ctx context.Context, input AnalyzeSpendingInput) (AdviceResponse, string, []byte, error) {
	prompt, err := buildAnalyzePrompt(input)
	if err != nil {
		return AdviceResponse{}, "", nil, err
	}

	messages := []Message{
		{Role: "system", Content: "You are a budgeting assistant. Respond with JSON only, without extra text."},
		{Role: "user", Content: prompt},
	}

	content, raw, err := s.client.Chat(ctx, messages)
	if err != nil {
		return AdviceResponse{}, prompt, raw, err
	}

	var response AdviceResponse
	if err := parseJSON(content, &response); err != nil {
		return AdviceResponse{}, prompt, raw, err
	}

	normalizeAdviceResponse(&response)
	if err := validateAdviceResponse(response); err != nil {
		return AdviceResponse{}, prompt, raw, err
	}

	return response, prompt, raw, nil
}

func buildGeneratePlanPrompt(input GeneratePlanInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`Create a structured budget plan as JSON.

Requirements:
- Output JSON only, no code fences, no extra text.
- Keep JSON compact (no extra whitespace).
- Use Russian (Cyrillic) for all titles and notes.
- Schema:
{
  "plan": {
    "title": string,
    "categories": [
      {
        "title": string,
        "type": "mandatory" | "optional",
        "items": [
          {"title": string, "amount_cents": integer, "priority": "red" | "yellow" | "green"}
        ]
      }
    ],
    "notes": [
      {"content": string, "type": "ai"}
    ]
  }
}
- Sum of all amount_cents must be <= budget_cents.
- Use integer amount_cents only.
- Provide exactly 2 categories and exactly 2 items per category.
- Provide 0-2 notes only.
- Keep titles short (<= 40 chars).

Input:
%s`, string(payload))

	return prompt, nil
}

func buildAnalyzePrompt(input AnalyzeSpendingInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`Analyze spending and return concise advice as JSON.

Requirements:
- Output JSON only, no code fences.
- Write advices in Russian (Cyrillic).
- Schema:
{
  "advices": [
    {"content": string, "type": "ai"}
  ]
}
- Provide 3-5 actionable advices.

Input:
%s`, string(payload))

	return prompt, nil
}

func parseJSON(input string, target interface{}) error {
	payload := extractJSON(input)
	if payload == "" {
		return errors.New("ai response does not contain json")
	}

	return json.Unmarshal([]byte(payload), target)
}

func extractJSON(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimPrefix(strings.TrimSpace(trimmed), "json")
		trimmed = strings.TrimSpace(trimmed)
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		trimmed = strings.TrimSpace(trimmed)
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}

	return trimmed[start : end+1]
}

func normalizePlanResponse(response *PlanResponse) {
	for i := range response.Plan.Notes {
		if strings.TrimSpace(response.Plan.Notes[i].Type) == "" {
			response.Plan.Notes[i].Type = noteTypeAI
		}
	}
}

func validatePlanResponse(response PlanResponse, budgetCents int64) error {
	if strings.TrimSpace(response.Plan.Title) == "" {
		return errors.New("plan title is required")
	}
	if len(response.Plan.Title) > 200 {
		return errors.New("plan title is too long")
	}

	if len(response.Plan.Categories) == 0 {
		return errors.New("plan categories are required")
	}
	if len(response.Plan.Categories) > 6 {
		return errors.New("too many categories")
	}

	var total int64
	itemCount := 0

	for _, category := range response.Plan.Categories {
		if strings.TrimSpace(category.Title) == "" {
			return errors.New("category title is required")
		}
		if len(category.Title) > 100 {
			return errors.New("category title is too long")
		}
		if !isCategoryType(category.Type) {
			return fmt.Errorf("invalid category type: %s", category.Type)
		}
		if len(category.Items) == 0 {
			return errors.New("each category must contain items")
		}

		for _, item := range category.Items {
			itemCount++
			if strings.TrimSpace(item.Title) == "" {
				return errors.New("item title is required")
			}
			if len(item.Title) > 200 {
				return errors.New("item title is too long")
			}
			if item.AmountCents <= 0 {
				return errors.New("item amount_cents must be positive")
			}
			if !isPriority(item.Priority) {
				return fmt.Errorf("invalid priority: %s", item.Priority)
			}
			total += item.AmountCents
		}
	}

	if itemCount < 4 {
		return errors.New("not enough items")
	}
	if itemCount > 20 {
		return errors.New("too many items")
	}

	if total > budgetCents {
		return errors.New("items exceed budget")
	}

	for _, note := range response.Plan.Notes {
		if strings.TrimSpace(note.Content) == "" {
			return errors.New("note content is required")
		}
		if !isNoteType(note.Type) {
			return fmt.Errorf("invalid note type: %s", note.Type)
		}
	}

	return nil
}

func normalizeAdviceResponse(response *AdviceResponse) {
	for i := range response.Advices {
		if strings.TrimSpace(response.Advices[i].Type) == "" {
			response.Advices[i].Type = noteTypeAI
		}
	}
}

func validateAdviceResponse(response AdviceResponse) error {
	if len(response.Advices) == 0 {
		return errors.New("advices are required")
	}

	for _, note := range response.Advices {
		if strings.TrimSpace(note.Content) == "" {
			return errors.New("advice content is required")
		}
		if !isNoteType(note.Type) {
			return fmt.Errorf("invalid advice type: %s", note.Type)
		}
	}

	return nil
}

func isCategoryType(value string) bool {
	switch strings.TrimSpace(value) {
	case mandatoryType, optionalType:
		return true
	default:
		return false
	}
}

func isPriority(value string) bool {
	switch strings.TrimSpace(value) {
	case priorityRed, priorityYellow, priorityGreen:
		return true
	default:
		return false
	}
}

func isNoteType(value string) bool {
	switch strings.TrimSpace(value) {
	case noteTypeAI, noteTypeUser:
		return true
	default:
		return false
	}
}
