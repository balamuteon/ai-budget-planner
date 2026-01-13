package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"example.com/ai-budget-planner/backend/internal/ai"
	"example.com/ai-budget-planner/backend/internal/auth"
	"example.com/ai-budget-planner/backend/internal/config"
	"example.com/ai-budget-planner/backend/internal/handlers"
	"example.com/ai-budget-planner/backend/internal/notifications"
	"example.com/ai-budget-planner/backend/internal/repository"
)

// New собирает HTTP-сервер Echo с роутами и зависимостями.
func New(cfg config.Config, logger *slog.Logger, db *pgxpool.Pool) *echo.Echo {
	if logger == nil {
		logger = slog.Default()
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = NewValidator()

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(requestLogger(logger))

	tokenManager := auth.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL)
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	planRepo := repository.NewPlanRepository(db)
	itemRepo := repository.NewItemRepository(db)
	noteRepo := repository.NewNoteRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	aiRepo := repository.NewAIRepository(db)
	adminRepo := repository.NewAdminRepository(db)
	notificationHub := notifications.NewHub()
	var aiClient ai.Client
	switch strings.ToLower(cfg.AI.Provider) {
	case "gemini":
		aiClient = ai.NewGeminiClient(cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.Model, cfg.AI.Timeout, cfg.AI.MaxOutputTokens)
	default:
		aiClient = ai.NewGroqClient(cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.Model, cfg.AI.Timeout, cfg.AI.MaxOutputTokens)
	}
	aiService := ai.NewService(aiClient)
	authHandler := handlers.NewAuthHandler(userRepo, tokenRepo, tokenManager)
	planHandler := handlers.NewPlanHandler(planRepo, notificationHub)
	itemHandler := handlers.NewItemHandler(itemRepo, planRepo, notificationHub)
	noteHandler := handlers.NewNoteHandler(noteRepo)
	statsHandler := handlers.NewStatsHandler(statsRepo)
	aiHandler := handlers.NewAIHandler(aiService, planRepo, noteRepo, aiRepo, notificationHub, cfg.AI.Provider, cfg.AI.Model)
	notificationHandler := handlers.NewNotificationHandler(notificationHub)
	adminHandler := handlers.NewAdminHandler(adminRepo)

	registerRoutes(
		e,
		authHandler,
		planHandler,
		itemHandler,
		noteHandler,
		statsHandler,
		aiHandler,
		notificationHandler,
		adminHandler,
		auth.JWTMiddleware(tokenManager),
		handlers.AdminMiddleware(userRepo, cfg.Admin.Emails),
		authRateLimiter(cfg.Auth),
		aiRateLimiter(cfg.AI),
	)

	return e
}

// NewHTTPServer создает net/http сервер с заданными таймаутами.
func NewHTTPServer(cfg config.ServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
}

func requestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:      true,
		LogStatus:   true,
		LogMethod:   true,
		LogLatency:  true,
		LogRemoteIP: true,
		LogError:    true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []slog.Attr{
				slog.String("method", v.Method),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.String("remote_ip", v.RemoteIP),
				slog.Duration("latency", v.Latency),
			}

			if v.Error != nil {
				attrs = append(attrs, slog.String("error", v.Error.Error()))
			}

			msg := "request completed"
			if v.Status >= http.StatusInternalServerError {
				logger.LogAttrs(c.Request().Context(), slog.LevelError, msg, attrs...)
				return nil
			}

			logger.LogAttrs(c.Request().Context(), slog.LevelInfo, msg, attrs...)
			return nil
		},
	})
}

func authRateLimiter(cfg config.AuthConfig) echo.MiddlewareFunc {
	limit := rate.Limit(float64(cfg.RateLimitPerMinute) / 60.0)
	store := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      limit,
		Burst:     cfg.RateLimitBurst,
		ExpiresIn: time.Minute,
	})

	return middleware.RateLimiter(store)
}

func aiRateLimiter(cfg config.AIConfig) echo.MiddlewareFunc {
	limit := rate.Limit(float64(cfg.RateLimitPerMinute) / 60.0)
	store := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      limit,
		Burst:     cfg.RateLimitBurst,
		ExpiresIn: time.Minute,
	})

	return middleware.RateLimiter(store)
}
