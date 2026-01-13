package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/models"
	"example.com/ai-budget-planner/backend/internal/notifications"
	"example.com/ai-budget-planner/backend/internal/repository"
)

type ItemHandler struct {
	Items    *repository.ItemRepository
	Plans    *repository.PlanRepository
	Notifier *notifications.Hub
}

// NewItemHandler создает обработчик операций с расходами.
func NewItemHandler(items *repository.ItemRepository, plans *repository.PlanRepository, notifier *notifications.Hub) *ItemHandler {
	return &ItemHandler{Items: items, Plans: plans, Notifier: notifier}
}

type CreateItemRequest struct {
	Title         string               `json:"title" validate:"required,max=200"`
	AmountCents   int64                `json:"amount_cents" validate:"gt=0"`
	PriorityColor models.PriorityColor `json:"priority_color" validate:"required,oneof=red yellow green"`
	IsCompleted   *bool                `json:"is_completed"`
}

type UpdateItemRequest struct {
	Title         string               `json:"title" validate:"required,max=200"`
	AmountCents   int64                `json:"amount_cents" validate:"gt=0"`
	PriorityColor models.PriorityColor `json:"priority_color" validate:"required,oneof=red yellow green"`
}

type ToggleItemRequest struct {
	IsCompleted *bool `json:"is_completed"`
}

type ReorderItemsRequest struct {
	ItemIDs []string `json:"item_ids" validate:"required,min=1"`
}

type UpdateColorRequest struct {
	PriorityColor models.PriorityColor `json:"priority_color" validate:"required,oneof=red yellow green"`
}

// Create добавляет новый расход в категорию плана.
func (h *ItemHandler) Create(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	categoryID, err := uuid.Parse(c.Param("categoryId"))
	if err != nil {
		return badRequest(c, "invalid category id")
	}

	var req CreateItemRequest
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

	isCompleted := false
	if req.IsCompleted != nil {
		isCompleted = *req.IsCompleted
	}

	item, err := h.Items.Create(c.Request().Context(), userID, planID, categoryID, title, req.AmountCents, req.PriorityColor, isCompleted)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan or category not found")
		}
		if errors.Is(err, repository.ErrBudgetExceeded) {
			return badRequest(c, "budget exceeded")
		}
		return serverError(c)
	}

	h.notifyBudgetUpdate(c.Request().Context(), userID, planID)
	return c.JSON(http.StatusCreated, toItemResponse(item))
}

// Update обновляет данные расхода.
func (h *ItemHandler) Update(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		return badRequest(c, "invalid item id")
	}

	planID, err := h.Items.GetPlanIDByItemID(c.Request().Context(), userID, itemID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "item not found")
		}
		return serverError(c)
	}

	var req UpdateItemRequest
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

	item, err := h.Items.Update(c.Request().Context(), userID, itemID, title, req.AmountCents, req.PriorityColor)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "item not found")
		}
		if errors.Is(err, repository.ErrBudgetExceeded) {
			return badRequest(c, "budget exceeded")
		}
		return serverError(c)
	}

	h.notifyBudgetUpdate(c.Request().Context(), userID, planID)
	return c.JSON(http.StatusOK, toItemResponse(item))
}

// Delete удаляет расход.
func (h *ItemHandler) Delete(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		return badRequest(c, "invalid item id")
	}

	planID, err := h.Items.GetPlanIDByItemID(c.Request().Context(), userID, itemID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "item not found")
		}
		return serverError(c)
	}

	if err := h.Items.Delete(c.Request().Context(), userID, itemID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "item not found")
		}
		return serverError(c)
	}

	h.notifyBudgetUpdate(c.Request().Context(), userID, planID)
	return c.NoContent(http.StatusNoContent)
}

// Toggle переключает статус выполнения расхода.
func (h *ItemHandler) Toggle(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		return badRequest(c, "invalid item id")
	}

	planID, err := h.Items.GetPlanIDByItemID(c.Request().Context(), userID, itemID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "item not found")
		}
		return serverError(c)
	}

	var req ToggleItemRequest
	if err = c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return badRequest(c, "invalid payload")
	}

	item, err := h.Items.Toggle(c.Request().Context(), userID, itemID, req.IsCompleted)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "item not found")
		}
		return serverError(c)
	}

	h.notifyBudgetUpdate(c.Request().Context(), userID, planID)
	return c.JSON(http.StatusOK, toItemResponse(item))
}

// Reorder меняет порядок расходов в категории.
func (h *ItemHandler) Reorder(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		return badRequest(c, "invalid item id")
	}

	var req ReorderItemsRequest
	if err = c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err = c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	itemIDs, err := parseUUIDs(req.ItemIDs)
	if err != nil {
		return badRequest(c, "invalid item ids")
	}

	if err := h.Items.Reorder(c.Request().Context(), userID, itemID, itemIDs); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "item not found")
		}
		if errors.Is(err, repository.ErrInvalid) {
			return badRequest(c, "invalid item order")
		}
		return serverError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

// UpdateColor обновляет цвет приоритета расхода.
func (h *ItemHandler) UpdateColor(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		return badRequest(c, "invalid item id")
	}

	var req UpdateColorRequest
	if err = c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err = c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	item, err := h.Items.UpdateColor(c.Request().Context(), userID, itemID, req.PriorityColor)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "item not found")
		}
		return serverError(c)
	}

	return c.JSON(http.StatusOK, toItemResponse(item))
}

func (h *ItemHandler) notifyBudgetUpdate(ctx context.Context, userID, planID uuid.UUID) {
	if h.Notifier == nil || h.Plans == nil {
		return
	}

	plan, err := h.Plans.GetByID(ctx, userID, planID)
	if err != nil {
		return
	}

	spent, err := h.Plans.GetSpentCents(ctx, plan.ID)
	if err != nil {
		return
	}

	publishBudgetUpdate(h.Notifier, userID, plan.ID, spent, plan.BudgetCents-spent)
}

func toItemResponse(item models.ExpenseItem) ItemResponse {
	return ItemResponse{
		ID:            item.ID,
		Title:         item.Title,
		AmountCents:   item.AmountCents,
		PriorityColor: item.PriorityColor,
		IsCompleted:   item.IsCompleted,
		SortOrder:     item.SortOrder,
	}
}
