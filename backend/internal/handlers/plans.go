package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/models"
	"example.com/ai-budget-planner/backend/internal/notifications"
	"example.com/ai-budget-planner/backend/internal/repository"
)

const (
	dateLayout             = "2006-01-02"
	defaultBackgroundColor = "#FDF7F7"
)

type PlanHandler struct {
	Plans    *repository.PlanRepository
	Notifier *notifications.Hub
}

// NewPlanHandler создает обработчик планов бюджета.
func NewPlanHandler(plans *repository.PlanRepository, notifier *notifications.Hub) *PlanHandler {
	return &PlanHandler{Plans: plans, Notifier: notifier}
}

type PlanRequest struct {
	Title           string  `json:"title" validate:"required,max=200"`
	BudgetCents     int64   `json:"budget_cents" validate:"gt=0"`
	PeriodStart     string  `json:"period_start" validate:"required"`
	PeriodEnd       string  `json:"period_end" validate:"required"`
	BackgroundColor *string `json:"background_color"`
	IsAIGenerated   *bool   `json:"is_ai_generated"`
}

type ReorderRequest struct {
	CategoryIDs []string `json:"category_ids" validate:"required,min=1"`
}

type PlanResponse struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	BudgetCents     int64     `json:"budget_cents"`
	PeriodStart     string    `json:"period_start"`
	PeriodEnd       string    `json:"period_end"`
	BackgroundColor string    `json:"background_color"`
	IsAIGenerated   bool      `json:"is_ai_generated"`
	SpentCents      int64     `json:"spent_cents"`
	RemainingCents  int64     `json:"remaining_cents"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CategoryResponse struct {
	ID           uuid.UUID           `json:"id"`
	Title        string              `json:"title"`
	CategoryType models.CategoryType `json:"category_type"`
	SortOrder    int                 `json:"sort_order"`
	Items        []ItemResponse      `json:"items"`
}

type ItemResponse struct {
	ID            uuid.UUID            `json:"id"`
	Title         string               `json:"title"`
	AmountCents   int64                `json:"amount_cents"`
	PriorityColor models.PriorityColor `json:"priority_color"`
	IsCompleted   bool                 `json:"is_completed"`
	SortOrder     int                  `json:"sort_order"`
}

type NoteResponse struct {
	ID        uuid.UUID       `json:"id"`
	Content   string          `json:"content"`
	NoteType  models.NoteType `json:"note_type"`
	SortOrder int             `json:"sort_order"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type PlanDetailResponse struct {
	Plan       PlanResponse       `json:"plan"`
	Categories []CategoryResponse `json:"categories"`
	Notes      []NoteResponse     `json:"notes"`
}

// List возвращает список планов пользователя.
func (h *PlanHandler) List(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	plans, err := h.Plans.ListByUser(c.Request().Context(), userID)
	if err != nil {
		return serverError(c)
	}

	response := make([]PlanResponse, 0, len(plans))
	for _, plan := range plans {
		response = append(response, toPlanResponse(plan.Plan, plan.SpentCents))
	}

	return c.JSON(http.StatusOK, map[string][]PlanResponse{"plans": response})
}

// Archive возвращает архивные планы пользователя.
func (h *PlanHandler) Archive(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	plans, err := h.Plans.ListArchivedByUser(c.Request().Context(), userID)
	if err != nil {
		return serverError(c)
	}

	response := make([]PlanResponse, 0, len(plans))
	for _, plan := range plans {
		response = append(response, toPlanResponse(plan.Plan, plan.SpentCents))
	}

	return c.JSON(http.StatusOK, map[string][]PlanResponse{"plans": response})
}

// Create создает новый план бюджета.
func (h *PlanHandler) Create(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	var req PlanRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return badRequest(c, "title is required")
	}

	periodStart, periodEnd, err := parsePeriod(req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return badRequest(c, err.Error())
	}

	backgroundColor := defaultBackgroundColor
	if req.BackgroundColor != nil {
		var value string
		value, err = validateHexColor(*req.BackgroundColor)
		if err != nil {
			return badRequest(c, err.Error())
		}
		backgroundColor = value
	}

	isAIGenerated := true
	if req.IsAIGenerated != nil {
		isAIGenerated = *req.IsAIGenerated
	}

	plan, err := h.Plans.Create(c.Request().Context(), userID, title, req.BudgetCents, periodStart, periodEnd, backgroundColor, isAIGenerated)
	if err != nil {
		return serverError(c)
	}

	response := toPlanResponse(plan, 0)
	publishBudgetUpdate(h.Notifier, userID, plan.ID, 0, response.RemainingCents)
	return c.JSON(http.StatusCreated, response)
}

// Get возвращает план по идентификатору.
func (h *PlanHandler) Get(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	plan, err := h.Plans.GetByID(c.Request().Context(), userID, planID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	response, err := buildPlanDetailResponse(c.Request().Context(), h.Plans, plan)
	if err != nil {
		return serverError(c)
	}

	return c.JSON(http.StatusOK, response)
}

// Update обновляет план бюджета.
func (h *PlanHandler) Update(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	var req PlanRequest
	if err = c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err = c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return badRequest(c, "title is required")
	}

	periodStart, periodEnd, err := parsePeriod(req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return badRequest(c, err.Error())
	}

	var backgroundColor *string
	if req.BackgroundColor != nil {
		var value string
		value, err = validateHexColor(*req.BackgroundColor)
		if err != nil {
			return badRequest(c, err.Error())
		}
		backgroundColor = &value
	}

	plan, err := h.Plans.Update(c.Request().Context(), userID, planID, title, req.BudgetCents, periodStart, periodEnd, backgroundColor, req.IsAIGenerated)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	spent, err := h.Plans.GetSpentCents(c.Request().Context(), plan.ID)
	if err != nil {
		return serverError(c)
	}

	response := toPlanResponse(plan, spent)
	publishBudgetUpdate(h.Notifier, userID, plan.ID, spent, response.RemainingCents)
	return c.JSON(http.StatusOK, response)
}

// Delete удаляет план бюджета.
func (h *PlanHandler) Delete(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	if err := h.Plans.Delete(c.Request().Context(), userID, planID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

// ReorderCategories меняет порядок категорий в плане.
func (h *PlanHandler) ReorderCategories(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	var req ReorderRequest
	if err = c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err = c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	categoryIDs, err := parseUUIDs(req.CategoryIDs)
	if err != nil {
		return badRequest(c, "invalid category ids")
	}

	if err := h.Plans.ReorderCategories(c.Request().Context(), userID, planID, categoryIDs); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan or categories not found")
		}
		if errors.Is(err, repository.ErrInvalid) {
			return badRequest(c, "invalid category order")
		}
		return serverError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

// Duplicate создает копию плана бюджета.
func (h *PlanHandler) Duplicate(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	plan, err := h.Plans.Duplicate(c.Request().Context(), userID, planID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	spent, err := h.Plans.GetSpentCents(c.Request().Context(), plan.ID)
	if err != nil {
		return serverError(c)
	}

	response := toPlanResponse(plan, spent)
	publishBudgetUpdate(h.Notifier, userID, plan.ID, spent, response.RemainingCents)
	return c.JSON(http.StatusCreated, response)
}

func parsePeriod(start, end string) (time.Time, time.Time, error) {
	periodStart, err := time.Parse(dateLayout, strings.TrimSpace(start))
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid period_start format")
	}

	periodEnd, err := time.Parse(dateLayout, strings.TrimSpace(end))
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid period_end format")
	}

	if periodEnd.Before(periodStart) {
		return time.Time{}, time.Time{}, errors.New("period_end must be after period_start")
	}

	return periodStart, periodEnd, nil
}

func validateHexColor(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("background_color is required")
	}
	if !isHexColor(trimmed) {
		return "", errors.New("background_color must be a hex color")
	}

	return trimmed, nil
}

func isHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}

	for i := 1; i < len(value); i++ {
		c := value[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}

	return true
}

func parseUUIDs(values []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))

	for _, value := range values {
		parsed, err := uuid.Parse(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}

		if _, exists := seen[parsed]; exists {
			return nil, errors.New("duplicate id")
		}

		seen[parsed] = struct{}{}
		ids = append(ids, parsed)
	}

	return ids, nil
}

func toPlanResponse(plan models.BudgetPlan, spentCents int64) PlanResponse {
	return PlanResponse{
		ID:              plan.ID,
		Title:           plan.Title,
		BudgetCents:     plan.BudgetCents,
		PeriodStart:     plan.PeriodStart.Format(dateLayout),
		PeriodEnd:       plan.PeriodEnd.Format(dateLayout),
		BackgroundColor: plan.BackgroundColor,
		IsAIGenerated:   plan.IsAIGenerated,
		SpentCents:      spentCents,
		RemainingCents:  plan.BudgetCents - spentCents,
		CreatedAt:       plan.CreatedAt,
		UpdatedAt:       plan.UpdatedAt,
	}
}
