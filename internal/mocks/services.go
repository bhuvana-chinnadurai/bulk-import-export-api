package mocks

import (
	"context"
	"net/http"

	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/service"
)

// MockImportService is a mock implementation of ImportService
type MockImportService struct {
	CreateJobFunc func(ctx context.Context, req *models.ImportRequest, filePath string) (*models.Job, error)
	ProcessFunc   func(ctx context.Context, job *models.Job) error
	ProcessedJobs []*models.Job
	CreatedJobs   []*models.Job
}

// Verify interface compliance
var _ service.ImportService = (*MockImportService)(nil)

func NewMockImportService() *MockImportService {
	return &MockImportService{
		ProcessedJobs: make([]*models.Job, 0),
		CreatedJobs:   make([]*models.Job, 0),
	}
}

func (m *MockImportService) CreateImportJob(ctx context.Context, req *models.ImportRequest, filePath string) (*models.Job, error) {
	if m.CreateJobFunc != nil {
		return m.CreateJobFunc(ctx, req, filePath)
	}
	job := &models.Job{
		ID:       "test-job-id",
		Resource: req.Resource,
		Status:   models.JobStatusPending,
	}
	m.CreatedJobs = append(m.CreatedJobs, job)
	return job, nil
}

func (m *MockImportService) ProcessImport(ctx context.Context, job *models.Job) error {
	if m.ProcessFunc != nil {
		return m.ProcessFunc(ctx, job)
	}
	m.ProcessedJobs = append(m.ProcessedJobs, job)
	job.Status = models.JobStatusCompleted
	return nil
}

// MockExportService is a mock implementation of ExportService
type MockExportService struct {
	StreamUsersFunc    func(ctx context.Context, w http.ResponseWriter, format string) error
	StreamArticlesFunc func(ctx context.Context, w http.ResponseWriter, format string) error
	StreamCommentsFunc func(ctx context.Context, w http.ResponseWriter, format string) error
	Counts             map[string]int
}

// Verify interface compliance
var _ service.ExportService = (*MockExportService)(nil)

func NewMockExportService() *MockExportService {
	return &MockExportService{
		Counts: map[string]int{
			"users":    0,
			"articles": 0,
			"comments": 0,
		},
	}
}

func (m *MockExportService) StreamUsers(ctx context.Context, w http.ResponseWriter, format string) error {
	if m.StreamUsersFunc != nil {
		return m.StreamUsersFunc(ctx, w, format)
	}
	return nil
}

func (m *MockExportService) StreamArticles(ctx context.Context, w http.ResponseWriter, format string) error {
	if m.StreamArticlesFunc != nil {
		return m.StreamArticlesFunc(ctx, w, format)
	}
	return nil
}

func (m *MockExportService) StreamComments(ctx context.Context, w http.ResponseWriter, format string) error {
	if m.StreamCommentsFunc != nil {
		return m.StreamCommentsFunc(ctx, w, format)
	}
	return nil
}

func (m *MockExportService) GetCount(ctx context.Context, resource string) (int, error) {
	return m.Counts[resource], nil
}

// MockJobService is a mock implementation of JobService
type MockJobService struct {
	Jobs          map[string]*models.JobResponse
	Errors        map[string][]models.ValidationError
	ImportService service.ImportService
}

// Verify interface compliance
var _ service.JobService = (*MockJobService)(nil)

func NewMockJobService() *MockJobService {
	return &MockJobService{
		Jobs:   make(map[string]*models.JobResponse),
		Errors: make(map[string][]models.ValidationError),
	}
}

func (m *MockJobService) StartProcessor(ctx context.Context) {}

func (m *MockJobService) StopProcessor() {}

func (m *MockJobService) GetJob(ctx context.Context, id string) (*models.JobResponse, error) {
	return m.Jobs[id], nil
}

func (m *MockJobService) GetJobByIdempotencyKey(ctx context.Context, key string) (*models.Job, error) {
	for _, job := range m.Jobs {
		if job.IdempotencyKey == key {
			return &job.Job, nil
		}
	}
	return nil, nil
}

func (m *MockJobService) GetJobErrors(ctx context.Context, id string) ([]models.ValidationError, error) {
	return m.Errors[id], nil
}

func (m *MockJobService) SetImportService(importService service.ImportService) {
	m.ImportService = importService
}
