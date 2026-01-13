package server

import (
	"github.com/labstack/echo/v4"

	"example.com/ai-budget-planner/backend/internal/handlers"
)

func registerRoutes(
	e *echo.Echo,
	authHandler *handlers.AuthHandler,
	planHandler *handlers.PlanHandler,
	itemHandler *handlers.ItemHandler,
	noteHandler *handlers.NoteHandler,
	statsHandler *handlers.StatsHandler,
	aiHandler *handlers.AIHandler,
	notificationHandler *handlers.NotificationHandler,
	adminHandler *handlers.AdminHandler,
	authMiddleware echo.MiddlewareFunc,
	adminMiddleware echo.MiddlewareFunc,
	authRateLimiter echo.MiddlewareFunc,
	aiRateLimiter echo.MiddlewareFunc,
) {
	e.GET("/health", handlers.Health)

	api := e.Group("/api/v1")
	authGroup := api.Group("/auth", authRateLimiter)

	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/logout", authHandler.Logout)
	authGroup.GET("/me", authHandler.Me, authMiddleware)

	plans := api.Group("/plans", authMiddleware)
	plans.GET("", planHandler.List)
	plans.GET("/archive", planHandler.Archive)
	plans.POST("", planHandler.Create)
	plans.GET("/:planId/notes", noteHandler.List)
	plans.POST("/:planId/notes", noteHandler.Create)
	plans.POST("/:planId/categories/:categoryId/items", itemHandler.Create)
	plans.PATCH("/:id/reorder", planHandler.ReorderCategories)
	plans.POST("/:id/duplicate", planHandler.Duplicate)
	plans.GET("/:id", planHandler.Get)
	plans.GET("/:id/export/json", planHandler.ExportJSON)
	plans.GET("/:id/export/csv", planHandler.ExportCSV)
	plans.PUT("/:id", planHandler.Update)
	plans.DELETE("/:id", planHandler.Delete)

	items := api.Group("/items", authMiddleware)
	items.PUT("/:itemId", itemHandler.Update)
	items.DELETE("/:itemId", itemHandler.Delete)
	items.PATCH("/:itemId/toggle", itemHandler.Toggle)
	items.PATCH("/:itemId/reorder", itemHandler.Reorder)
	items.PATCH("/:itemId/color", itemHandler.UpdateColor)

	notes := api.Group("/notes", authMiddleware)
	notes.PUT("/:noteId", noteHandler.Update)
	notes.DELETE("/:noteId", noteHandler.Delete)
	notes.PATCH("/:noteId/reorder", noteHandler.Reorder)

	stats := api.Group("/stats", authMiddleware)
	stats.GET("/overview", statsHandler.Overview)
	stats.GET("/spending-by-category", statsHandler.SpendingByCategory)
	stats.GET("/monthly-comparison", statsHandler.MonthlyComparison)

	notifications := api.Group("/notifications", authMiddleware)
	notifications.GET("/stream", notificationHandler.Stream)

	admin := api.Group("/admin", authMiddleware, adminMiddleware)
	admin.GET("/users", adminHandler.ListUsers)
	admin.GET("/ai-requests", adminHandler.ListAIRequests)
	admin.GET("/usage", adminHandler.Usage)

	aiGroup := api.Group("/ai", authMiddleware, aiRateLimiter)
	aiGroup.POST("/generate-plan", aiHandler.GeneratePlan)
	aiGroup.POST("/analyze-spending", aiHandler.AnalyzeSpending)
	aiGroup.GET("/advices/:planId", aiHandler.GetAdvices)
}
