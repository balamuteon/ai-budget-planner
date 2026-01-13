package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/repository"
)

type AdminHandler struct {
	Repo *repository.AdminRepository
}

// NewAdminHandler создает обработчик админских эндпоинтов.
func NewAdminHandler(repo *repository.AdminRepository) *AdminHandler {
	return &AdminHandler{Repo: repo}
}

type AdminUserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      *string   `json:"name,omitempty"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

type AdminUsersResponse struct {
	Total int                 `json:"total"`
	Users []AdminUserResponse `json:"users"`
}

type AdminAIRequestResponse struct {
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	RequestType     string          `json:"request_type"`
	Provider        string          `json:"provider"`
	Model           string          `json:"model"`
	Success         bool            `json:"success"`
	ErrorMessage    *string         `json:"error_message,omitempty"`
	CreatedAt       string          `json:"created_at"`
	Prompt          *string         `json:"prompt,omitempty"`
	RequestPayload  json.RawMessage `json:"request_payload,omitempty"`
	ResponsePayload json.RawMessage `json:"response_payload,omitempty"`
	RawResponse     *string         `json:"raw_response,omitempty"`
}

type AdminAIRequestsResponse struct {
	Total    int                      `json:"total"`
	Requests []AdminAIRequestResponse `json:"requests"`
}

type AdminUsageDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type AdminUsageResponse struct {
	Users           int             `json:"users"`
	Plans           int             `json:"plans"`
	AIRequests      int             `json:"ai_requests"`
	AISuccess       int             `json:"ai_success"`
	AIFail          int             `json:"ai_fail"`
	AIRequestsByDay []AdminUsageDay `json:"ai_requests_by_day"`
}

// ListUsers возвращает список пользователей для админки.
func (h *AdminHandler) ListUsers(c echo.Context) error {
	limit, offset, err := parsePagination(c, 50, 200)
	if err != nil {
		return badRequest(c, err.Error())
	}

	users, err := h.Repo.ListUsers(c.Request().Context(), limit, offset)
	if err != nil {
		return serverError(c)
	}

	total, err := h.Repo.CountUsers(c.Request().Context())
	if err != nil {
		return serverError(c)
	}

	response := make([]AdminUserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, AdminUserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Format(timeLayout),
			UpdatedAt: user.UpdatedAt.Format(timeLayout),
		})
	}

	return c.JSON(http.StatusOK, AdminUsersResponse{
		Total: total,
		Users: response,
	})
}

// ListAIRequests возвращает логи AI-запросов с фильтрами.
func (h *AdminHandler) ListAIRequests(c echo.Context) error {
	limit, offset, err := parsePagination(c, 50, 200)
	if err != nil {
		return badRequest(c, err.Error())
	}

	filter := repository.AIRequestFilter{}
	if raw := strings.TrimSpace(c.QueryParam("user_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return badRequest(c, "invalid user_id")
		}
		filter.UserID = &parsed
	}

	if raw := strings.TrimSpace(c.QueryParam("success")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return badRequest(c, "invalid success")
		}
		filter.Success = &parsed
	}

	if raw := strings.TrimSpace(c.QueryParam("request_type")); raw != "" {
		filter.RequestType = &raw
	}

	includePayloads := false
	if raw := strings.TrimSpace(c.QueryParam("include_payloads")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return badRequest(c, "invalid include_payloads")
		}
		includePayloads = parsed
	}

	requests, err := h.Repo.ListAIRequests(c.Request().Context(), filter, limit, offset, includePayloads)
	if err != nil {
		return serverError(c)
	}

	total, err := h.Repo.CountAIRequests(c.Request().Context(), filter)
	if err != nil {
		return serverError(c)
	}

	response := make([]AdminAIRequestResponse, 0, len(requests))
	for _, req := range requests {
		item := AdminAIRequestResponse{
			ID:           req.ID,
			UserID:       req.UserID,
			RequestType:  req.RequestType,
			Provider:     req.Provider,
			Model:        req.Model,
			Success:      req.Success,
			ErrorMessage: req.ErrorMessage,
			CreatedAt:    req.CreatedAt.Format(timeLayout),
		}

		if includePayloads {
			item.Prompt = req.Prompt
			if len(req.RequestPayload) > 0 {
				item.RequestPayload = json.RawMessage(req.RequestPayload)
			}
			if len(req.ResponsePayload) > 0 {
				item.ResponsePayload = json.RawMessage(req.ResponsePayload)
			}
			item.RawResponse = req.RawResponse
		}
		response = append(response, item)
	}

	return c.JSON(http.StatusOK, AdminAIRequestsResponse{
		Total:    total,
		Requests: response,
	})
}

// Usage возвращает агрегированную статистику использования.
func (h *AdminHandler) Usage(c echo.Context) error {
	days := 7
	if raw := strings.TrimSpace(c.QueryParam("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return badRequest(c, "invalid days")
		}
		if parsed > 30 {
			parsed = 30
		}
		days = parsed
	}

	stats, err := h.Repo.UsageStats(c.Request().Context(), days)
	if err != nil {
		if errors.Is(err, repository.ErrInvalid) {
			return badRequest(c, "invalid days")
		}
		return serverError(c)
	}

	daysResponse := make([]AdminUsageDay, 0, len(stats.AIRequestsByDay))
	for _, day := range stats.AIRequestsByDay {
		daysResponse = append(daysResponse, AdminUsageDay{
			Date:  day.Day.Format("2006-01-02"),
			Count: day.Count,
		})
	}

	return c.JSON(http.StatusOK, AdminUsageResponse{
		Users:           stats.Users,
		Plans:           stats.Plans,
		AIRequests:      stats.AIRequests,
		AISuccess:       stats.AISuccess,
		AIFail:          stats.AIFail,
		AIRequestsByDay: daysResponse,
	})
}

// AdminMiddleware ограничивает доступ к админским роутам по email.
func AdminMiddleware(users *repository.UserRepository, emails []string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		trimmed := strings.ToLower(strings.TrimSpace(email))
		if trimmed == "" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := auth.UserIDFromContext(c)
			if !ok {
				return unauthorized(c)
			}

			if len(allowed) == 0 {
				return forbidden(c)
			}

			user, err := users.GetByID(c.Request().Context(), userID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return forbidden(c)
				}
				return serverError(c)
			}

			email := strings.ToLower(strings.TrimSpace(user.Email))
			if _, ok := allowed[email]; !ok {
				return forbidden(c)
			}

			return next(c)
		}
	}
}

func parsePagination(c echo.Context, defaultLimit, maxLimit int) (int, int, error) {
	limit := defaultLimit
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return 0, 0, errors.New("invalid limit")
		}
		if parsed > maxLimit {
			parsed = maxLimit
		}
		limit = parsed
	}

	offset := 0
	if raw := strings.TrimSpace(c.QueryParam("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return 0, 0, errors.New("invalid offset")
		}
		offset = parsed
	}

	return limit, offset, nil
}
