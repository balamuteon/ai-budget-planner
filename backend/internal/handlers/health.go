package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type HealthResponse struct {
	Status string `json:"status"`
}

// Health возвращает простой статус сервиса.
func Health(c echo.Context) error {
	return c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}
