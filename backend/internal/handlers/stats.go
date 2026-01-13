package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/repository"
)

type StatsHandler struct {
	Stats *repository.StatsRepository
}

// NewStatsHandler создает обработчик статистики.
func NewStatsHandler(stats *repository.StatsRepository) *StatsHandler {
	return &StatsHandler{Stats: stats}
}

type OverviewResponse struct {
	TotalPlans       int   `json:"total_plans"`
	ActivePlans      int   `json:"active_plans"`
	ArchivedPlans    int   `json:"archived_plans"`
	TotalBudgetCents int64 `json:"total_budget_cents"`
	TotalSpentCents  int64 `json:"total_spent_cents"`
	RemainingCents   int64 `json:"remaining_cents"`
}

type CategorySpendingResponse struct {
	PlanID     uuid.UUID                  `json:"plan_id"`
	Categories []CategorySpendingCategory `json:"categories"`
}

type CategorySpendingCategory struct {
	CategoryID   uuid.UUID `json:"category_id"`
	Title        string    `json:"title"`
	CategoryType string    `json:"category_type"`
	SpentCents   int64     `json:"spent_cents"`
}

type MonthlyComparisonResponse struct {
	Months []MonthlyComparisonItem `json:"months"`
}

type MonthlyComparisonItem struct {
	Month       string `json:"month"`
	BudgetCents int64  `json:"budget_cents"`
	SpentCents  int64  `json:"spent_cents"`
}

// Overview возвращает сводную статистику по планам.
func (h *StatsHandler) Overview(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	stats, err := h.Stats.Overview(c.Request().Context(), userID)
	if err != nil {
		return serverError(c)
	}

	return c.JSON(http.StatusOK, OverviewResponse{
		TotalPlans:       stats.TotalPlans,
		ActivePlans:      stats.ActivePlans,
		ArchivedPlans:    stats.ArchivedPlans,
		TotalBudgetCents: stats.TotalBudgetCents,
		TotalSpentCents:  stats.TotalSpentCents,
		RemainingCents:   stats.TotalBudgetCents - stats.TotalSpentCents,
	})
}

// SpendingByCategory возвращает траты по категориям в плане.
func (h *StatsHandler) SpendingByCategory(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planIDParam := c.QueryParam("plan_id")
	if planIDParam == "" {
		return badRequest(c, "plan_id is required")
	}

	planID, err := uuid.Parse(planIDParam)
	if err != nil {
		return badRequest(c, "invalid plan_id")
	}

	items, err := h.Stats.SpendingByCategory(c.Request().Context(), userID, planID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	categories := make([]CategorySpendingCategory, 0, len(items))
	for _, item := range items {
		categories = append(categories, CategorySpendingCategory{
			CategoryID:   item.CategoryID,
			Title:        item.Title,
			CategoryType: string(item.CategoryType),
			SpentCents:   item.SpentCents,
		})
	}

	return c.JSON(http.StatusOK, CategorySpendingResponse{
		PlanID:     planID,
		Categories: categories,
	})
}

// MonthlyComparison возвращает сравнение по месяцам.
func (h *StatsHandler) MonthlyComparison(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	months := 6
	if raw := c.QueryParam("months"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return badRequest(c, "invalid months")
		}
		if parsed > 24 {
			parsed = 24
		}
		months = parsed
	}

	items, err := h.Stats.MonthlyComparison(c.Request().Context(), userID, months)
	if err != nil {
		if errors.Is(err, repository.ErrInvalid) {
			return badRequest(c, "invalid months")
		}
		return serverError(c)
	}

	response := make([]MonthlyComparisonItem, 0, len(items))
	for _, item := range items {
		response = append(response, MonthlyComparisonItem{
			Month:       item.Month.Format("2006-01"),
			BudgetCents: item.BudgetCents,
			SpentCents:  item.SpentCents,
		})
	}

	return c.JSON(http.StatusOK, MonthlyComparisonResponse{Months: response})
}
