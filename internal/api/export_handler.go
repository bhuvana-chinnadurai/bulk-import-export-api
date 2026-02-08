package api

import (
	"net/http"

	"github.com/bulk-import-export-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// ExportHandler handles export endpoints
type ExportHandler struct {
	services *service.Services
	log      zerolog.Logger
}

// NewExportHandler creates a new ExportHandler
func NewExportHandler(services *service.Services, log zerolog.Logger) *ExportHandler {
	return &ExportHandler{
		services: services,
		log:      log.With().Str("handler", "export").Logger(),
	}
}

// StreamExport handles GET /v1/exports?resource=...&format=...
// Streams the export directly to the response
func (h *ExportHandler) StreamExport(c *gin.Context) {
	ctx := c.Request.Context()

	resource := c.Query("resource")
	if resource == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource parameter is required (users, articles, comments)"})
		return
	}
	if resource != "users" && resource != "articles" && resource != "comments" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource must be one of: users, articles, comments"})
		return
	}

	format := c.Query("format")
	if format == "" {
		format = "ndjson" // Default to NDJSON for streaming
	}
	if format != "ndjson" && format != "json" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be one of: ndjson, json, csv"})
		return
	}

	// CSV only supported for users
	if format == "csv" && resource != "users" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV format only supported for users export"})
		return
	}

	h.log.Info().
		Str("resource", resource).
		Str("format", format).
		Msg("Starting streaming export")

	var err error
	switch resource {
	case "users":
		err = h.services.Export.StreamUsers(ctx, c.Writer, format)
	case "articles":
		err = h.services.Export.StreamArticles(ctx, c.Writer, format)
	case "comments":
		err = h.services.Export.StreamComments(ctx, c.Writer, format)
	}

	if err != nil {
		h.log.Error().Err(err).Str("resource", resource).Msg("Export failed")
		// Can't return error JSON after streaming has started
		return
	}
}

// CreateExport handles POST /v1/exports
// Creates an async export job (for large exports with filters)
func (h *ExportHandler) CreateExport(c *gin.Context) {
	var req struct {
		Resource string            `json:"resource"`
		Format   string            `json:"format"`
		Filters  map[string]string `json:"filters,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Resource == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource is required"})
		return
	}
	if req.Resource != "users" && req.Resource != "articles" && req.Resource != "comments" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource must be one of: users, articles, comments"})
		return
	}

	if req.Format == "" {
		req.Format = "ndjson"
	}

	// For now, redirect to streaming export
	// In a full implementation, this would create an async job
	h.log.Info().
		Str("resource", req.Resource).
		Str("format", req.Format).
		Interface("filters", req.Filters).
		Msg("Async export requested - redirecting to streaming")

	c.JSON(http.StatusOK, gin.H{
		"message":    "For streaming export, use GET /v1/exports?resource=" + req.Resource + "&format=" + req.Format,
		"resource":   req.Resource,
		"format":     req.Format,
		"stream_url": "/v1/exports?resource=" + req.Resource + "&format=" + req.Format,
	})
}

// GetExportStatus handles GET /v1/exports/:job_id
func (h *ExportHandler) GetExportStatus(c *gin.Context) {
	ctx := c.Request.Context()
	jobID := c.Param("job_id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	job, err := h.services.Job.GetJob(ctx, jobID)
	if err != nil {
		h.log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get job status"})
		return
	}
	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}
