package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/models"
	"example.com/ai-budget-planner/backend/internal/repository"
)

type NoteHandler struct {
	Notes *repository.NoteRepository
}

// NewNoteHandler создает обработчик заметок.
func NewNoteHandler(notes *repository.NoteRepository) *NoteHandler {
	return &NoteHandler{Notes: notes}
}

type NoteRequest struct {
	Content  string          `json:"content" validate:"required"`
	NoteType models.NoteType `json:"note_type" validate:"required,oneof=ai user"`
}

type ReorderNotesRequest struct {
	NoteIDs []string `json:"note_ids" validate:"required,min=1"`
}

// List возвращает заметки плана.
func (h *NoteHandler) List(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	notes, err := h.Notes.ListByPlan(c.Request().Context(), userID, planID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	response := make([]NoteResponse, 0, len(notes))
	for _, note := range notes {
		response = append(response, toNoteResponse(note))
	}

	return c.JSON(http.StatusOK, map[string][]NoteResponse{"notes": response})
}

// Create добавляет заметку к плану.
func (h *NoteHandler) Create(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	planID, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		return badRequest(c, "invalid plan id")
	}

	var req NoteRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return badRequest(c, "content is required")
	}

	note, err := h.Notes.Create(c.Request().Context(), userID, planID, content, req.NoteType)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "plan not found")
		}
		return serverError(c)
	}

	return c.JSON(http.StatusCreated, toNoteResponse(note))
}

// Update обновляет заметку.
func (h *NoteHandler) Update(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	noteID, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		return badRequest(c, "invalid note id")
	}

	var req NoteRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return badRequest(c, "content is required")
	}

	note, err := h.Notes.Update(c.Request().Context(), userID, noteID, content, req.NoteType)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "note not found")
		}
		return serverError(c)
	}

	return c.JSON(http.StatusOK, toNoteResponse(note))
}

// Delete удаляет заметку.
func (h *NoteHandler) Delete(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	noteID, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		return badRequest(c, "invalid note id")
	}

	if err := h.Notes.Delete(c.Request().Context(), userID, noteID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "note not found")
		}
		return serverError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

// Reorder меняет порядок заметок.
func (h *NoteHandler) Reorder(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	noteID, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		return badRequest(c, "invalid note id")
	}

	var req ReorderNotesRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	noteIDs, err := parseUUIDs(req.NoteIDs)
	if err != nil {
		return badRequest(c, "invalid note ids")
	}

	if err := h.Notes.Reorder(c.Request().Context(), userID, noteID, noteIDs); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound(c, "note not found")
		}
		if errors.Is(err, repository.ErrInvalid) {
			return badRequest(c, "invalid note order")
		}
		return serverError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

func toNoteResponse(note models.Note) NoteResponse {
	return NoteResponse{
		ID:        note.ID,
		Content:   note.Content,
		NoteType:  note.NoteType,
		SortOrder: note.SortOrder,
		CreatedAt: note.CreatedAt,
		UpdatedAt: note.UpdatedAt,
	}
}
