package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/notifications"
)

type NotificationHandler struct {
	Hub *notifications.Hub
}

// NewNotificationHandler создает SSE-обработчик уведомлений.
func NewNotificationHandler(hub *notifications.Hub) *NotificationHandler {
	return &NotificationHandler{Hub: hub}
}

// Stream открывает SSE-поток событий для пользователя.
func (h *NotificationHandler) Stream(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return serverError(c)
	}

	ch, unsubscribe := h.Hub.Subscribe(userID)
	defer unsubscribe()

	_ = writeSSE(c, notifications.Event{Type: "connected", Data: map[string]string{"user_id": userID.String()}})
	flusher.Flush()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := writeSSE(c, event); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}

func writeSSE(c echo.Context, event notifications.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if _, err := c.Response().Write([]byte("event: " + event.Type + "\n")); err != nil {
		return err
	}
	if _, err := c.Response().Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
		return err
	}

	return nil
}

func publishBudgetUpdate(hub *notifications.Hub, userID uuid.UUID, planID uuid.UUID, spentCents int64, remainingCents int64) {
	if hub == nil {
		return
	}

	hub.Publish(userID, notifications.Event{
		Type: "budget_updated",
		Data: map[string]interface{}{
			"plan_id":         planID.String(),
			"spent_cents":     spentCents,
			"remaining_cents": remainingCents,
		},
	})
}

func publishAdviceUpdate(hub *notifications.Hub, userID uuid.UUID, planID uuid.UUID, count int) {
	if hub == nil {
		return
	}

	hub.Publish(userID, notifications.Event{
		Type: "ai_advices",
		Data: map[string]interface{}{
			"plan_id": planID.String(),
			"count":   count,
		},
	})
}
