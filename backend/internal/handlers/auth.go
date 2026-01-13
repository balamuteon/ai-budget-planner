package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/models"
	"example.com/ai-budget-planner/backend/internal/repository"
)

type AuthHandler struct {
	Users        *repository.UserRepository
	Tokens       *repository.RefreshTokenRepository
	TokenManager *auth.TokenManager
}

// NewAuthHandler создает обработчик авторизации.
func NewAuthHandler(users *repository.UserRepository, tokens *repository.RefreshTokenRepository, manager *auth.TokenManager) *AuthHandler {
	return &AuthHandler{
		Users:        users,
		Tokens:       tokens,
		TokenManager: manager,
	}
}

type RegisterRequest struct {
	Email    string  `json:"email" validate:"required,email"`
	Password string  `json:"password" validate:"required,min=8"`
	Name     *string `json:"name" validate:"omitempty,max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type AuthUser struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  *string   `json:"name,omitempty"`
}

type AuthResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	User         AuthUser `json:"user"`
}

type UserResponse struct {
	User AuthUser `json:"user"`
}

// Register регистрирует пользователя и выдает токены.
func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	name := normalizeName(req.Name)

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return serverError(c)
	}

	user, err := h.Users.Create(c.Request().Context(), email, passwordHash, name)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return conflict(c, "user already exists")
		}
		return serverError(c)
	}

	response, err := h.issueTokens(c.Request().Context(), user)
	if err != nil {
		return serverError(c)
	}

	return c.JSON(http.StatusCreated, response)
}

// Login выполняет вход и выдает токены.
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)

	user, err := h.Users.GetByEmail(c.Request().Context(), email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return unauthorized(c)
		}
		return serverError(c)
	}

	if err = auth.ComparePassword(user.PasswordHash, password); err != nil {
		return unauthorized(c)
	}

	response, err := h.issueTokens(c.Request().Context(), user)
	if err != nil {
		return serverError(c)
	}

	return c.JSON(http.StatusOK, response)
}

// Refresh обновляет токены по refresh-токену.
func (h *AuthHandler) Refresh(c echo.Context) error {
	var req RefreshRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	claims, err := h.TokenManager.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		return unauthorized(c)
	}

	refreshID, err := uuid.Parse(claims.ID)
	if err != nil {
		return unauthorized(c)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return unauthorized(c)
	}

	storedToken, err := h.Tokens.GetByID(c.Request().Context(), refreshID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return unauthorized(c)
		}
		return serverError(c)
	}

	if storedToken.RevokedAt != nil || time.Now().After(storedToken.ExpiresAt) {
		return unauthorized(c)
	}

	if storedToken.UserID != userID {
		return unauthorized(c)
	}

	if !auth.CompareTokenHash(storedToken.TokenHash, req.RefreshToken) {
		return unauthorized(c)
	}

	newRefreshID := uuid.New()
	tokenPair, err := h.TokenManager.NewTokenPair(userID, newRefreshID)
	if err != nil {
		return serverError(c)
	}

	user, err := h.Users.GetByID(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return unauthorized(c)
		}
		return serverError(c)
	}

	newToken := models.RefreshToken{
		ID:        newRefreshID,
		UserID:    userID,
		TokenHash: auth.HashToken(tokenPair.RefreshToken),
		ExpiresAt: tokenPair.RefreshExpiresAt,
	}

	if err := h.Tokens.Rotate(c.Request().Context(), storedToken.ID, newToken); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return unauthorized(c)
		}
		return serverError(c)
	}

	return c.JSON(http.StatusOK, AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         toAuthUser(user),
	})
}

// Logout отзывает refresh-токен.
func (h *AuthHandler) Logout(c echo.Context) error {
	var req LogoutRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid payload")
	}
	if err := c.Validate(&req); err != nil {
		return badRequest(c, "validation failed")
	}

	claims, err := h.TokenManager.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		return unauthorized(c)
	}

	refreshID, err := uuid.Parse(claims.ID)
	if err != nil {
		return unauthorized(c)
	}

	if err := h.Tokens.Revoke(c.Request().Context(), refreshID, nil); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.NoContent(http.StatusNoContent)
		}
		return serverError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

// Me возвращает данные текущего пользователя.
func (h *AuthHandler) Me(c echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return unauthorized(c)
	}

	user, err := h.Users.GetByID(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return serverError(c)
	}

	return c.JSON(http.StatusOK, UserResponse{User: toAuthUser(user)})
}

func (h *AuthHandler) issueTokens(ctx context.Context, user models.User) (AuthResponse, error) {
	refreshID := uuid.New()
	pair, err := h.TokenManager.NewTokenPair(user.ID, refreshID)
	if err != nil {
		return AuthResponse{}, err
	}

	refreshToken := models.RefreshToken{
		ID:        refreshID,
		UserID:    user.ID,
		TokenHash: auth.HashToken(pair.RefreshToken),
		ExpiresAt: pair.RefreshExpiresAt,
	}

	if err := h.Tokens.Create(ctx, refreshToken); err != nil {
		return AuthResponse{}, err
	}

	return AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         toAuthUser(user),
	}, nil
}

func toAuthUser(user models.User) AuthUser {
	return AuthUser{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	}
}

func normalizeName(name *string) *string {
	if name == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*name)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func badRequest(c echo.Context, message string) error {
	return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
}

func unauthorized(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
}

func conflict(c echo.Context, message string) error {
	return c.JSON(http.StatusConflict, map[string]string{"error": message})
}

func notFound(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, map[string]string{"error": message})
}

func forbidden(c echo.Context) error {
	return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
}

func serverError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
