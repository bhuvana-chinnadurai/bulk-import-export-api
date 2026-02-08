package service

import (
	"context"
	"net/http"

	"github.com/bulk-import-export-api/internal/config"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/repository"
	"github.com/rs/zerolog"
)

// ImportService defines the interface for import operations
type ImportService interface {
	CreateImportJob(ctx context.Context, req *models.ImportRequest, filePath string) (*models.Job, error)
	ProcessImport(ctx context.Context, job *models.Job) error
}

// ExportService defines the interface for export operations
type ExportService interface {
	StreamUsers(ctx context.Context, w http.ResponseWriter, format string) error
	StreamArticles(ctx context.Context, w http.ResponseWriter, format string) error
	StreamComments(ctx context.Context, w http.ResponseWriter, format string) error
	GetCount(ctx context.Context, resource string) (int, error)
}

// JobService defines the interface for job management
type JobService interface {
	StartProcessor(ctx context.Context)
	StopProcessor()
	GetJob(ctx context.Context, id string) (*models.JobResponse, error)
	GetJobByIdempotencyKey(ctx context.Context, key string) (*models.Job, error)
	GetJobErrors(ctx context.Context, id string) ([]models.ValidationError, error)
	SetImportService(importService ImportService)
}

// Services holds all service interfaces
type Services struct {
	Import ImportService
	Export ExportService
	Job    JobService
}

// NewServices creates all services
func NewServices(repos *repository.Repositories, cfg *config.Config, log zerolog.Logger) *Services {
	jobSvc := newJobService(repos.Job, log)
	importSvc := newImportService(repos, jobSvc, cfg, log)
	exportSvc := newExportService(repos, log)

	// Wire up job processor to import service
	jobSvc.SetImportService(importSvc)

	return &Services{
		Import: importSvc,
		Export: exportSvc,
		Job:    jobSvc,
	}
}
