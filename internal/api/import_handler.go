package api

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bulk-import-export-api/internal/config"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ImportHandler handles import endpoints
type ImportHandler struct {
	services *service.Services
	cfg      *config.Config
	log      zerolog.Logger
}

// NewImportHandler creates a new ImportHandler
func NewImportHandler(services *service.Services, cfg *config.Config, log zerolog.Logger) *ImportHandler {
	return &ImportHandler{
		services: services,
		cfg:      cfg,
		log:      log.With().Str("handler", "import").Logger(),
	}
}

// CreateImport handles POST /v1/imports
// Accepts file upload (multipart) or JSON body with file URL
func (h *ImportHandler) CreateImport(c *gin.Context) {
	ctx := c.Request.Context()

	// Get idempotency key from header
	idempotencyKey := c.GetHeader("Idempotency-Key")

	// Check for existing job with same idempotency key
	if idempotencyKey != "" {
		existingJob, err := h.services.Job.GetJobByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			h.log.Error().Err(err).Msg("Failed to check idempotency key")
		}
		if existingJob != nil {
			h.log.Info().Str("job_id", existingJob.ID).Msg("Returning existing job for idempotency key")
			c.JSON(http.StatusOK, existingJob)
			return
		}
	}

	// Get resource type
	resource := c.PostForm("resource")
	if resource == "" {
		resource = c.Query("resource")
	}
	if resource == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource parameter is required (users, articles, comments)"})
		return
	}
	if resource != "users" && resource != "articles" && resource != "comments" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource must be one of: users, articles, comments"})
		return
	}

	// Handle file upload
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		// Try JSON body with file URL
		var req struct {
			FileURL  string `json:"file_url"`
			Resource string `json:"resource"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file upload or file_url is required"})
			return
		}

		// TODO: Implement URL download
		c.JSON(http.StatusNotImplemented, gin.H{"error": "file_url not yet implemented, please use file upload"})
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size > h.cfg.Import.MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("file too large, max size is %d MB", h.cfg.Import.MaxUploadSize/(1024*1024)),
		})
		return
	}

	// Determine file format from extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if resource == "users" && ext != ".csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "users import requires CSV file"})
		return
	}
	if (resource == "articles" || resource == "comments") && ext != ".ndjson" && ext != ".json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "articles/comments import requires NDJSON file"})
		return
	}

	// Save uploaded file
	uploadDir := h.cfg.Import.UploadDir
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		h.log.Error().Err(err).Msg("Failed to create upload directory")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	filename := fmt.Sprintf("%s_%s%s", resource, uuid.New().String()[:8], ext)
	filePath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to create file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		h.log.Error().Err(err).Msg("Failed to copy file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	// Create import job
	req := &models.ImportRequest{
		Resource:       resource,
		IdempotencyKey: idempotencyKey,
	}

	job, err := h.services.Import.CreateImportJob(ctx, req, filePath)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to create import job")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create import job"})
		return
	}

	h.log.Info().
		Str("job_id", job.ID).
		Str("resource", resource).
		Str("file", header.Filename).
		Int64("size_bytes", header.Size).
		Msg("Import job created")

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":   job.ID,
		"status":   job.Status,
		"resource": job.Resource,
		"message":  "Import job created and queued for processing",
	})
}

// GetImportStatus handles GET /v1/imports/:job_id
func (h *ImportHandler) GetImportStatus(c *gin.Context) {
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

// GetImportErrors handles GET /v1/imports/:job_id/errors
func (h *ImportHandler) GetImportErrors(c *gin.Context) {
	ctx := c.Request.Context()
	jobID := c.Param("job_id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	errors, err := h.services.Job.GetJobErrors(ctx, jobID)
	if err != nil {
		h.log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job errors")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get errors"})
		return
	}

	// Determine format from query param
	format := c.Query("format")
	if format == "" {
		format = "json"
	}

	if format == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=errors_%s.csv", jobID))
		writer := csv.NewWriter(c.Writer)
		writer.Write([]string{"line", "field", "message", "value"})
		for _, e := range errors {
			value := ""
			if e.Value != nil {
				value = fmt.Sprintf("%v", e.Value)
			}
			writer.Write([]string{strconv.Itoa(e.Line), e.Field, e.Message, value})
		}
		writer.Flush()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id":      jobID,
		"error_count": len(errors),
		"errors":      errors,
	})
}
