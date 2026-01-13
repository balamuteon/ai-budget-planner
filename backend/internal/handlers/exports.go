package handlers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/repository"
)

const (
	exportTypeItems = "items"
	exportTypeNotes = "notes"
)

const timeLayout = time.RFC3339

// ExportJSON выгружает план в JSON-файл.
func (h *PlanHandler) ExportJSON(c echo.Context) error {
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

	filename := "plan-" + plan.ID.String() + ".json"
	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+filename+"\"")
	return c.JSON(http.StatusOK, response)
}

// ExportCSV выгружает план в CSV-файл.
func (h *PlanHandler) ExportCSV(c echo.Context) error {
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

	exportType := strings.ToLower(strings.TrimSpace(c.QueryParam("type")))
	if exportType == "" {
		exportType = exportTypeItems
	}

	response, err := buildPlanDetailResponse(c.Request().Context(), h.Plans, plan)
	if err != nil {
		return serverError(c)
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	switch exportType {
	case exportTypeNotes:
		if err := writeNotesCSV(writer, response); err != nil {
			return serverError(c)
		}
	case exportTypeItems:
		if err := writeItemsCSV(writer, response); err != nil {
			return serverError(c)
		}
	default:
		return badRequest(c, "invalid export type")
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return serverError(c)
	}

	filename := "plan-" + plan.ID.String() + "-" + exportType + ".csv"
	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+filename+"\"")
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

func writeItemsCSV(writer *csv.Writer, response PlanDetailResponse) error {
	header := []string{
		"plan_id",
		"plan_title",
		"category_id",
		"category_title",
		"category_type",
		"item_id",
		"item_title",
		"amount_cents",
		"priority_color",
		"is_completed",
		"sort_order",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, category := range response.Categories {
		for _, item := range category.Items {
			record := []string{
				response.Plan.ID.String(),
				response.Plan.Title,
				category.ID.String(),
				category.Title,
				string(category.CategoryType),
				item.ID.String(),
				item.Title,
				formatInt64(item.AmountCents),
				string(item.PriorityColor),
				formatBool(item.IsCompleted),
				formatInt(item.SortOrder),
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeNotesCSV(writer *csv.Writer, response PlanDetailResponse) error {
	header := []string{
		"plan_id",
		"plan_title",
		"note_id",
		"content",
		"note_type",
		"sort_order",
		"created_at",
		"updated_at",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, note := range response.Notes {
		record := []string{
			response.Plan.ID.String(),
			response.Plan.Title,
			note.ID.String(),
			note.Content,
			string(note.NoteType),
			formatInt(note.SortOrder),
			note.CreatedAt.Format(timeLayout),
			note.UpdatedAt.Format(timeLayout),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func formatInt(value int) string {
	return strconv.Itoa(value)
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
