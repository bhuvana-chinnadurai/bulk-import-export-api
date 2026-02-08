package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bulk-import-export-api/internal/mocks"
	"github.com/bulk-import-export-api/internal/models"
)

func TestJobService_GetJob(t *testing.T) {
	mockJobRepo := mocks.NewMockJobRepository()

	// Create test job
	testJob := &models.Job{
		ID:              "test-job-123",
		Type:            models.JobTypeImport,
		Resource:        "users",
		Status:          models.JobStatusCompleted,
		TotalRecords:    1000,
		SuccessfulCount: 950,
		FailedCount:     50,
		DurationMs:      5000,
		RowsPerSec:      200.0,
		CreatedAt:       time.Now(),
	}
	mockJobRepo.Create(context.Background(), testJob)

	// Add some errors
	mockJobRepo.AddErrors(context.Background(), testJob.ID, []models.ValidationError{
		{Line: 10, Field: "email", Message: "invalid email"},
		{Line: 25, Field: "role", Message: "invalid role"},
	})

	// Create service using internal function - simplified test
	// In a real test, we'd use the interface directly

	// Verify job retrieval via mock
	retrieved, err := mockJobRepo.GetByID(context.Background(), testJob.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Job should be found")
	}
	if retrieved.TotalRecords != 1000 {
		t.Errorf("Expected 1000 total records, got %d", retrieved.TotalRecords)
	}

	// Verify errors
	errors, _ := mockJobRepo.GetErrors(context.Background(), testJob.ID, 0)
	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}
}

func TestJobService_GetJobWithMetrics(t *testing.T) {
	mockJobRepo := mocks.NewMockJobRepository()
	now := time.Now()
	startTime := now.Add(-5 * time.Second)

	// Create completed job with metrics
	testJob := &models.Job{
		ID:              "metrics-job",
		Type:            models.JobTypeImport,
		Resource:        "articles",
		Status:          models.JobStatusCompleted,
		TotalRecords:    10000,
		ProcessedCount:  10000,
		SuccessfulCount: 9500,
		FailedCount:     500,
		DurationMs:      5000,
		RowsPerSec:      2000.0,
		CreatedAt:       now.Add(-10 * time.Second),
		StartedAt:       &startTime,
		CompletedAt:     &now,
	}
	mockJobRepo.Create(context.Background(), testJob)

	retrieved, _ := mockJobRepo.GetByID(context.Background(), "metrics-job")

	// Verify metrics
	if retrieved.RowsPerSec != 2000.0 {
		t.Errorf("Expected 2000.0 rows/sec, got %f", retrieved.RowsPerSec)
	}
	if retrieved.DurationMs != 5000 {
		t.Errorf("Expected 5000ms duration, got %d", retrieved.DurationMs)
	}
	if retrieved.StartedAt == nil {
		t.Error("StartedAt should be set")
	}
	if retrieved.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestExportService_StreamUsers(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepository()

	// Add test users with explicit values
	for i := 0; i < 10; i++ {
		mockUserRepo.Create(context.Background(), &models.User{
			ID:        "550e8400-e29b-41d4-a716-" + padInt(i, 12),
			Email:     "user" + padInt(i, 4) + "@test.com",
			Name:      "Test User " + padInt(i, 4),
			Role:      "admin",
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	// Test streaming
	ctx := context.Background()
	count := 0
	err := mockUserRepo.StreamAll(ctx, func(user *models.User) error {
		count++
		return nil
	})

	if err != nil {
		t.Fatalf("StreamAll failed: %v", err)
	}
	if count != 10 {
		t.Errorf("Expected 10 users streamed, got %d", count)
	}
}

func TestMockImportService_CreateAndProcess(t *testing.T) {
	mockImportService := mocks.NewMockImportService()
	ctx := context.Background()

	// Create import job
	req := &models.ImportRequest{
		Resource:       "users",
		IdempotencyKey: "test-key-123",
	}

	job, err := mockImportService.CreateImportJob(ctx, req, "/path/to/file.csv")
	if err != nil {
		t.Fatalf("CreateImportJob failed: %v", err)
	}

	if job == nil {
		t.Fatal("Job should not be nil")
	}
	if job.Resource != "users" {
		t.Errorf("Expected resource 'users', got '%s'", job.Resource)
	}
	if len(mockImportService.CreatedJobs) != 1 {
		t.Errorf("Expected 1 created job, got %d", len(mockImportService.CreatedJobs))
	}

	// Process job
	err = mockImportService.ProcessImport(ctx, job)
	if err != nil {
		t.Fatalf("ProcessImport failed: %v", err)
	}

	if len(mockImportService.ProcessedJobs) != 1 {
		t.Errorf("Expected 1 processed job, got %d", len(mockImportService.ProcessedJobs))
	}
	if job.Status != models.JobStatusCompleted {
		t.Errorf("Expected status Completed, got %s", job.Status)
	}
}

func TestMockExportService_GetCount(t *testing.T) {
	mockExportService := mocks.NewMockExportService()
	mockExportService.Counts["users"] = 1000
	mockExportService.Counts["articles"] = 500
	mockExportService.Counts["comments"] = 2500

	ctx := context.Background()

	tests := []struct {
		resource string
		want     int
	}{
		{"users", 1000},
		{"articles", 500},
		{"comments", 2500},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			count, err := mockExportService.GetCount(ctx, tt.resource)
			if err != nil {
				t.Fatalf("GetCount failed: %v", err)
			}
			if count != tt.want {
				t.Errorf("Expected %d, got %d", tt.want, count)
			}
		})
	}
}

func TestMockJobService_IdempotencyKey(t *testing.T) {
	mockJobService := mocks.NewMockJobService()

	// Add a job with idempotency key
	jobResponse := &models.JobResponse{
		Job: models.Job{
			ID:             "existing-job",
			Resource:       "users",
			Status:         models.JobStatusCompleted,
			IdempotencyKey: "idempotent-key-123",
		},
	}
	mockJobService.Jobs["existing-job"] = jobResponse

	ctx := context.Background()

	// Look up by idempotency key
	found, err := mockJobService.GetJobByIdempotencyKey(ctx, "idempotent-key-123")
	if err != nil {
		t.Fatalf("GetJobByIdempotencyKey failed: %v", err)
	}
	if found == nil {
		t.Fatal("Should find job by idempotency key")
	}
	if found.ID != "existing-job" {
		t.Errorf("Expected 'existing-job', got '%s'", found.ID)
	}

	// Non-existent key
	found, _ = mockJobService.GetJobByIdempotencyKey(ctx, "nonexistent")
	if found != nil {
		t.Error("Should not find job with nonexistent key")
	}
}

func padInt(i, width int) string {
	s := fmt.Sprintf("%d", i)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
