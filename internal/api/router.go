package api

import (
	"context"
	"net/http"
	"time"

	"github.com/bulk-import-export-api/internal/config"
	"github.com/bulk-import-export-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// NewRouter creates and configures the Gin router
func NewRouter(services *service.Services, cfg *config.Config, log zerolog.Logger) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Middleware
	router.Use(recoveryMiddleware(log))
	router.Use(loggingMiddleware(log))
	router.Use(corsMiddleware())

	// Handlers
	importHandler := NewImportHandler(services, cfg, log)
	exportHandler := NewExportHandler(services, log)

	// Health check
	router.GET("/health", healthCheck)
	router.GET("/metrics", metricsHandler(services))

	// API v1
	v1 := router.Group("/v1")
	{
		// Import endpoints
		imports := v1.Group("/imports")
		{
			imports.POST("", importHandler.CreateImport)
			imports.GET("/:job_id", importHandler.GetImportStatus)
			imports.GET("/:job_id/errors", importHandler.GetImportErrors)
		}

		// Export endpoints
		exports := v1.Group("/exports")
		{
			exports.GET("", exportHandler.StreamExport)
			exports.POST("", exportHandler.CreateExport)
			exports.GET("/:job_id", exportHandler.GetExportStatus)
		}
	}

	return router
}

// healthCheck returns the health status
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "bulk-import-export-api",
	})
}

// metricsHandler returns import/export metrics
func metricsHandler(services *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usersCount, _ := services.Export.GetCount(ctx, "users")
		articlesCount, _ := services.Export.GetCount(ctx, "articles")
		commentsCount, _ := services.Export.GetCount(ctx, "comments")

		c.JSON(http.StatusOK, gin.H{
			"database": gin.H{
				"users":    usersCount,
				"articles": articlesCount,
				"comments": commentsCount,
			},
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// recoveryMiddleware handles panics
func recoveryMiddleware(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error().Interface("error", err).Msg("Panic recovered")
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// loggingMiddleware logs requests
func loggingMiddleware(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		event := log.Info()
		if statusCode >= 400 {
			event = log.Warn()
		}
		if statusCode >= 500 {
			event = log.Error()
		}

		event.
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", statusCode).
			Dur("duration", duration).
			Str("client_ip", c.ClientIP()).
			Msg("Request completed")
	}
}

// corsMiddleware handles CORS
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// contextWithTimeout creates a context with timeout for handlers
func contextWithTimeout(c *gin.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), timeout)
}
