package auth

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const ContextUserIDKey = "user_id"

// JWTMiddleware проверяет access-токен и сохраняет user_id в контексте.
func JWTMiddleware(manager *TokenManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header")
			}

			tokenString := strings.TrimSpace(parts[1])
			if tokenString == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header")
			}

			claims, err := manager.ParseAccessToken(tokenString)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token subject")
			}

			c.Set(ContextUserIDKey, userID)
			return next(c)
		}
	}
}

// UserIDFromContext извлекает идентификатор пользователя из контекста.
func UserIDFromContext(c echo.Context) (uuid.UUID, bool) {
	value := c.Get(ContextUserIDKey)
	userID, ok := value.(uuid.UUID)
	return userID, ok
}
