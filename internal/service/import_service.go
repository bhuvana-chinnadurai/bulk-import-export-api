package service

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bulk-import-export-api/internal/config"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/repository"
	"github.com/bulk-import-export-api/internal/validation"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// importService is the concrete implementation of ImportService
type importService struct {
	repos      *repository.Repositories
	jobService JobService
	cfg        *config.Config
	log        zerolog.Logger
}

// newImportService creates a new ImportService
func newImportService(repos *repository.Repositories, jobService JobService, cfg *config.Config, log zerolog.Logger) *importService {
	return &importService{
		repos:      repos,
		jobService: jobService,
		cfg:        cfg,
		log:        log.With().Str("service", "import").Logger(),
	}
}

// CreateImportJob creates a new import job
func (s *importService) CreateImportJob(ctx context.Context, req *models.ImportRequest, filePath string) (*models.Job, error) {
	job := &models.Job{
		ID:             uuid.New().String(),
		Type:           models.JobTypeImport,
		Resource:       req.Resource,
		Status:         models.JobStatusPending,
		IdempotencyKey: req.IdempotencyKey,
		FilePath:       filePath,
		CreatedAt:      time.Now(),
	}

	if err := s.repos.Job.Create(ctx, job); err != nil {
		return nil, err
	}

	s.log.Info().
		Str("job_id", job.ID).
		Str("resource", job.Resource).
		Str("file", filePath).
		Msg("Import job created")

	return job, nil
}

// ProcessImport processes an import job
func (s *importService) ProcessImport(ctx context.Context, job *models.Job) error {
	startTime := time.Now()
	now := startTime
	job.Status = models.JobStatusProcessing
	job.StartedAt = &now
	s.repos.Job.Update(ctx, job)

	s.log.Info().
		Str("job_id", job.ID).
		Str("resource", job.Resource).
		Msg("Starting import processing")

	var err error
	switch job.Resource {
	case "users":
		err = s.processUsersCSV(ctx, job)
	case "articles":
		err = s.processArticlesNDJSON(ctx, job)
	case "comments":
		err = s.processCommentsNDJSON(ctx, job)
	default:
		err = fmt.Errorf("unknown resource type: %s", job.Resource)
	}

	// Calculate metrics
	duration := time.Since(startTime)
	job.DurationMs = duration.Milliseconds()
	if job.ProcessedCount > 0 && duration.Seconds() > 0 {
		job.RowsPerSec = float64(job.ProcessedCount) / duration.Seconds()
	}

	completedAt := time.Now()
	job.CompletedAt = &completedAt

	// Calculate error rate for observability
	var errorRate float64
	if job.TotalRecords > 0 {
		errorRate = float64(job.FailedCount) / float64(job.TotalRecords) * 100
	}

	if err != nil {
		job.Status = models.JobStatusFailed
		s.log.Error().Err(err).Str("job_id", job.ID).Msg("Import failed")
	} else {
		job.Status = models.JobStatusCompleted
		s.log.Info().
			Str("job_id", job.ID).
			Int("total", job.TotalRecords).
			Int("successful", job.SuccessfulCount).
			Int("failed", job.FailedCount).
			Float64("error_rate_pct", errorRate).
			Int64("duration_ms", job.DurationMs).
			Float64("rows_per_sec", job.RowsPerSec).
			Msg("Import completed")
	}

	s.repos.Job.Update(ctx, job)

	return err
}

// processUsersCSV processes a users CSV file
func (s *importService) processUsersCSV(ctx context.Context, job *models.Job) error {
	file, err := os.Open(job.FilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	validator := validation.NewValidator()
	batchSize := s.cfg.Import.BatchSize

	// Read header
	header, err := reader.Read()
	if err != nil {
		return err
	}
	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	var batch []*models.User
	var validationErrors []models.ValidationError
	lineNum := 1 // Start after header

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		lineNum++
		job.TotalRecords++

		// Respect context cancellation for long-running imports
		if lineNum%10000 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		// Parse CSV row into UserCSV struct
		userCSV := &models.UserCSV{
			ID:        getField(record, headerMap, "id"),
			Email:     getField(record, headerMap, "email"),
			Name:      getField(record, headerMap, "name"),
			Role:      getField(record, headerMap, "role"),
			Active:    getField(record, headerMap, "active"),
			CreatedAt: getField(record, headerMap, "created_at"),
			UpdatedAt: getField(record, headerMap, "updated_at"),
		}

		// Validate
		errors := validator.ValidateUser(userCSV, lineNum)
		if len(errors) > 0 {
			job.FailedCount++
			job.ProcessedCount++
			for _, e := range errors {
				validationErrors = append(validationErrors, models.ValidationError{
					Line:    lineNum,
					Field:   e.Field,
					Message: e.Message,
					Value:   e.Value,
				})
			}
			// Flush errors periodically to prevent unbounded memory growth
			if len(validationErrors) >= errorFlushThreshold {
				s.flushValidationErrors(ctx, job.ID, &validationErrors)
			}
			continue
		}

		// Convert to User model
		user := convertCSVToUser(userCSV)
		batch = append(batch, user)
		validator.AddUserEmail(userCSV.Email)
		validator.AddUserID(userCSV.ID)

		// Process batch
		if len(batch) >= batchSize {
			inserted, err := s.repos.User.BatchInsert(ctx, batch)
			if err != nil {
				s.log.Error().Err(err).Int("batch_size", len(batch)).Msg("Batch insert failed")
				job.FailedCount += len(batch)
			} else {
				job.SuccessfulCount += inserted
			}
			job.ProcessedCount += len(batch)
			batch = batch[:0]

			s.log.Debug().
				Str("job_id", job.ID).
				Int("processed", job.ProcessedCount).
				Float64("rows_per_sec", float64(job.ProcessedCount)/time.Since(*job.StartedAt).Seconds()).
				Msg("Batch processed")
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		inserted, err := s.repos.User.BatchInsert(ctx, batch)
		if err != nil {
			s.log.Error().Err(err).Int("batch_size", len(batch)).Msg("Batch insert failed")
			job.FailedCount += len(batch)
		} else {
			job.SuccessfulCount += inserted
		}
		job.ProcessedCount += len(batch)
	}

	// Store validation errors
	if len(validationErrors) > 0 {
		s.repos.Job.AddErrors(ctx, job.ID, validationErrors)
	}

	return nil
}

// processArticlesNDJSON processes articles NDJSON file
func (s *importService) processArticlesNDJSON(ctx context.Context, job *models.Job) error {
	file, err := os.Open(job.FilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	validator := validation.NewValidator()
	batchSize := s.cfg.Import.BatchSize

	// Pre-load user IDs for FK validation (if not too many)
	userIDs, _ := s.repos.User.GetAllIDs(ctx)
	if len(userIDs) < 100000 {
		validator.SetUserIDCache(userIDs)
	}

	var batch []*models.Article
	var validationErrors []models.ValidationError
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		job.TotalRecords++

		// Respect context cancellation for long-running imports
		if lineNum%10000 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		var article models.ArticleNDJSON
		if err := json.Unmarshal([]byte(line), &article); err != nil {
			job.FailedCount++
			job.ProcessedCount++
			validationErrors = append(validationErrors, models.ValidationError{
				Line:    lineNum,
				Field:   "json",
				Message: fmt.Sprintf("invalid JSON: %v", err),
			})
			if len(validationErrors) >= errorFlushThreshold {
				s.flushValidationErrors(ctx, job.ID, &validationErrors)
			}
			continue
		}

		// Validate
		errors := validator.ValidateArticle(&article, lineNum)
		if len(errors) > 0 {
			job.FailedCount++
			job.ProcessedCount++
			for _, e := range errors {
				validationErrors = append(validationErrors, models.ValidationError{
					Line:    lineNum,
					Field:   e.Field,
					Message: e.Message,
					Value:   e.Value,
				})
			}
			if len(validationErrors) >= errorFlushThreshold {
				s.flushValidationErrors(ctx, job.ID, &validationErrors)
			}
			continue
		}

		// Convert to Article model
		articleModel := convertNDJSONToArticle(&article)
		batch = append(batch, articleModel)
		validator.AddArticleSlug(article.Slug)
		validator.AddArticleID(article.ID)

		// Process batch
		if len(batch) >= batchSize {
			inserted, err := s.repos.Article.BatchInsert(ctx, batch)
			if err != nil {
				s.log.Error().Err(err).Int("batch_size", len(batch)).Msg("Batch insert failed")
				job.FailedCount += len(batch)
			} else {
				job.SuccessfulCount += inserted
			}
			job.ProcessedCount += len(batch)
			batch = batch[:0]

			s.log.Debug().
				Str("job_id", job.ID).
				Int("processed", job.ProcessedCount).
				Float64("rows_per_sec", float64(job.ProcessedCount)/time.Since(*job.StartedAt).Seconds()).
				Msg("Batch processed")
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		inserted, err := s.repos.Article.BatchInsert(ctx, batch)
		if err != nil {
			s.log.Error().Err(err).Int("batch_size", len(batch)).Msg("Batch insert failed")
			job.FailedCount += len(batch)
		} else {
			job.SuccessfulCount += inserted
		}
		job.ProcessedCount += len(batch)
	}

	// Store validation errors
	if len(validationErrors) > 0 {
		s.repos.Job.AddErrors(ctx, job.ID, validationErrors)
	}

	return scanner.Err()
}

// processCommentsNDJSON processes comments NDJSON file
func (s *importService) processCommentsNDJSON(ctx context.Context, job *models.Job) error {
	file, err := os.Open(job.FilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	validator := validation.NewValidator()
	batchSize := s.cfg.Import.BatchSize

	// Pre-load IDs for FK validation
	userIDs, _ := s.repos.User.GetAllIDs(ctx)
	articleIDs, _ := s.repos.Article.GetAllIDs(ctx)
	if len(userIDs) < 100000 {
		validator.SetUserIDCache(userIDs)
	}
	if len(articleIDs) < 100000 {
		validator.SetArticleIDCache(articleIDs)
	}

	var batch []*models.Comment
	var validationErrors []models.ValidationError
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		job.TotalRecords++

		// Respect context cancellation for long-running imports
		if lineNum%10000 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		var comment models.CommentNDJSON
		if err := json.Unmarshal([]byte(line), &comment); err != nil {
			job.FailedCount++
			job.ProcessedCount++
			validationErrors = append(validationErrors, models.ValidationError{
				Line:    lineNum,
				Field:   "json",
				Message: fmt.Sprintf("invalid JSON: %v", err),
			})
			if len(validationErrors) >= errorFlushThreshold {
				s.flushValidationErrors(ctx, job.ID, &validationErrors)
			}
			continue
		}

		// Validate
		errors := validator.ValidateComment(&comment, lineNum)
		if len(errors) > 0 {
			job.FailedCount++
			job.ProcessedCount++
			for _, e := range errors {
				validationErrors = append(validationErrors, models.ValidationError{
					Line:    lineNum,
					Field:   e.Field,
					Message: e.Message,
					Value:   e.Value,
				})
			}
			if len(validationErrors) >= errorFlushThreshold {
				s.flushValidationErrors(ctx, job.ID, &validationErrors)
			}
			continue
		}

		// Convert to Comment model
		commentModel := convertNDJSONToComment(&comment)
		batch = append(batch, commentModel)

		// Process batch
		if len(batch) >= batchSize {
			inserted, err := s.repos.Comment.BatchInsert(ctx, batch)
			if err != nil {
				s.log.Error().Err(err).Int("batch_size", len(batch)).Msg("Batch insert failed")
				job.FailedCount += len(batch)
			} else {
				job.SuccessfulCount += inserted
			}
			job.ProcessedCount += len(batch)
			batch = batch[:0]

			s.log.Debug().
				Str("job_id", job.ID).
				Int("processed", job.ProcessedCount).
				Float64("rows_per_sec", float64(job.ProcessedCount)/time.Since(*job.StartedAt).Seconds()).
				Msg("Batch processed")
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		inserted, err := s.repos.Comment.BatchInsert(ctx, batch)
		if err != nil {
			s.log.Error().Err(err).Int("batch_size", len(batch)).Msg("Batch insert failed")
			job.FailedCount += len(batch)
		} else {
			job.SuccessfulCount += inserted
		}
		job.ProcessedCount += len(batch)
	}

	// Store validation errors
	if len(validationErrors) > 0 {
		s.repos.Job.AddErrors(ctx, job.ID, validationErrors)
	}

	return scanner.Err()
}

// flushValidationErrors writes accumulated errors to the database and resets the slice.
// This prevents unbounded memory growth: at 1M records with 100% error rate,
// each error is ~200 bytes → flushing every 1000 caps memory at ~200KB instead of ~200MB.
const errorFlushThreshold = 1000

func (s *importService) flushValidationErrors(ctx context.Context, jobID string, errors *[]models.ValidationError) {
	if len(*errors) == 0 {
		return
	}
	if err := s.repos.Job.AddErrors(ctx, jobID, *errors); err != nil {
		s.log.Error().Err(err).Int("count", len(*errors)).Msg("Failed to flush validation errors")
	}
	*errors = (*errors)[:0]
}

// Helper functions

func getField(record []string, headerMap map[string]int, field string) string {
	if idx, ok := headerMap[field]; ok && idx < len(record) {
		return strings.TrimSpace(record[idx])
	}
	return ""
}

func convertCSVToUser(csv *models.UserCSV) *models.User {
	createdAt, _ := time.Parse(time.RFC3339, csv.CreatedAt)
	return &models.User{
		ID:        csv.ID,
		Email:     csv.Email,
		Name:      csv.Name,
		Role:      csv.Role,
		Active:    csv.Active == "true",
		CreatedAt: createdAt,
	}
}

func convertNDJSONToArticle(ndjson *models.ArticleNDJSON) *models.Article {
	article := &models.Article{
		ID:       ndjson.ID,
		Slug:     ndjson.Slug,
		Title:    ndjson.Title,
		Body:     ndjson.Body,
		AuthorID: ndjson.AuthorID,
		Tags:     ndjson.Tags,
		Status:   ndjson.Status,
	}
	if article.Status == "" {
		article.Status = "draft"
	}
	if ndjson.PublishedAt != "" {
		t, _ := time.Parse(time.RFC3339, ndjson.PublishedAt)
		article.PublishedAt = &t
	}
	article.CreatedAt = time.Now()
	return article
}

func convertNDJSONToComment(ndjson *models.CommentNDJSON) *models.Comment {
	createdAt, _ := time.Parse(time.RFC3339, ndjson.CreatedAt)
	return &models.Comment{
		ID:        ndjson.ID,
		ArticleID: ndjson.ArticleID,
		UserID:    ndjson.UserID,
		Body:      ndjson.Body,
		CreatedAt: createdAt,
	}
}
