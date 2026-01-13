package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/ai-budget-planner/backend/internal/config"
	"example.com/ai-budget-planner/backend/internal/database"
	"example.com/ai-budget-planner/backend/internal/server"
)

func main() {
	ensureEnvFile()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		db.Close()
	}()

	e := server.New(cfg, logger, db)
	httpServer := server.NewHTTPServer(cfg.Server, e)

	go func() {
		if err := e.StartServer(httpServer); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", slog.String("error", err.Error()))
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
	}
}

func ensureEnvFile() {
	if os.Getenv("ENV_FILE") != "" {
		return
	}

	if _, err := os.Stat(".env"); err == nil {
		_ = os.Setenv("ENV_FILE", ".env")
		return
	}

	if _, err := os.Stat("../.env"); err == nil {
		_ = os.Setenv("ENV_FILE", "../.env")
	}
}
