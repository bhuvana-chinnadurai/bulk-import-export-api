package service_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bulk-import-export-api/internal/config"
	"github.com/bulk-import-export-api/internal/mocks"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/repository"
	"github.com/bulk-import-export-api/internal/service"
	"github.com/rs/zerolog"
)

// testdataPath returns the absolute path to a file in the testdata directory.
func testdataPath(t testing.TB, filename string) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
	path := filepath.Join(projectRoot, "testdata", filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("testdata file not found: %s", path)
	}
	return path
}

type testHarness struct {
	services    *service.Services
	userRepo    *mocks.MockUserRepository
	articleRepo *mocks.MockArticleRepository
	commentRepo *mocks.MockCommentRepository
	jobRepo     *mocks.MockJobRepository
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()

	userRepo := mocks.NewMockUserRepository()
	articleRepo := mocks.NewMockArticleRepository()
	commentRepo := mocks.NewMockCommentRepository()
	jobRepo := mocks.NewMockJobRepository()

	repos := &repository.Repositories{
		User:    userRepo,
		Article: articleRepo,
		Comment: commentRepo,
		Job:     jobRepo,
	}

	cfg := &config.Config{
		Import: config.ImportConfig{
			BatchSize:     1000,
			MaxUploadSize: 500 * 1024 * 1024,
			UploadDir:     os.TempDir(),
		},
	}

	log := zerolog.Nop()
	services := service.NewServices(repos, cfg, log)

	return &testHarness{
		services:    services,
		userRepo:    userRepo,
		articleRepo: articleRepo,
		commentRepo: commentRepo,
		jobRepo:     jobRepo,
	}
}

func createTestJob(h *testHarness, resource, filePath string) *models.Job {
	now := time.Now()
	job := &models.Job{
		ID:        "integration-test-job",
		Type:      models.JobTypeImport,
		Resource:  resource,
		Status:    models.JobStatusPending,
		FilePath:  filePath,
		StartedAt: &now,
		CreatedAt: now,
	}
	h.jobRepo.Create(context.Background(), job)
	return job
}

// --- Users CSV Integration Tests ---

func TestProcessImport_UsersCSV_HugeFile(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "users_huge.csv")

	job := createTestJob(h, "users", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	// File has 10001 lines (1 header + 10000 data rows)
	if job.TotalRecords != 10000 {
		t.Errorf("Expected 10000 total records, got %d", job.TotalRecords)
	}

	// Verify some records failed validation
	if job.FailedCount == 0 {
		t.Error("Expected some failed records (invalid emails, duplicate emails, invalid roles, missing IDs)")
	}

	// Verify successful + failed = total
	if job.SuccessfulCount+job.FailedCount != job.TotalRecords {
		t.Errorf("SuccessfulCount(%d) + FailedCount(%d) = %d, but TotalRecords = %d",
			job.SuccessfulCount, job.FailedCount, job.SuccessfulCount+job.FailedCount, job.TotalRecords)
	}

	// Verify ProcessedCount = TotalRecords
	if job.ProcessedCount != job.TotalRecords {
		t.Errorf("ProcessedCount(%d) should equal TotalRecords(%d)", job.ProcessedCount, job.TotalRecords)
	}

	// Verify records were actually inserted into mock repo
	if len(h.userRepo.Users) == 0 {
		t.Error("Expected some users to be inserted into repository")
	}
	if len(h.userRepo.Users) != job.SuccessfulCount {
		t.Errorf("Repository has %d users, but SuccessfulCount is %d", len(h.userRepo.Users), job.SuccessfulCount)
	}

	// Verify validation errors were stored
	storedErrors := h.jobRepo.Errors[job.ID]
	if len(storedErrors) == 0 {
		t.Error("Expected validation errors to be stored in job repository")
	}

	// Verify error details contain expected fields
	errorFields := make(map[string]bool)
	for _, e := range storedErrors {
		errorFields[e.Field] = true
	}
	expectedFields := []string{"id", "email", "role"}
	for _, field := range expectedFields {
		if !errorFields[field] {
			t.Errorf("Expected validation errors for field '%s', but none found", field)
		}
	}

	// Verify job status is completed
	if job.Status != models.JobStatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", job.Status)
	}

	// Verify metrics
	if job.DurationMs <= 0 {
		t.Error("Expected positive duration_ms")
	}
	if job.RowsPerSec <= 0 {
		t.Error("Expected positive rows_per_sec")
	}

	t.Logf("Users CSV: total=%d, successful=%d, failed=%d, errors=%d, batches=%d, duration=%dms, rows/sec=%.0f",
		job.TotalRecords, job.SuccessfulCount, job.FailedCount, len(storedErrors),
		h.userRepo.BatchInsertCalls, job.DurationMs, job.RowsPerSec)
}

func TestProcessImport_UsersCSV_ContinueOnError(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "users_huge.csv")

	job := createTestJob(h, "users", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	// Even with errors, processing should not stop
	if job.SuccessfulCount == 0 {
		t.Error("Continue-on-error: Expected some successful records despite validation failures")
	}
	if job.FailedCount == 0 {
		t.Error("Continue-on-error: Expected some failed records")
	}
	if job.TotalRecords != 10000 {
		t.Errorf("Continue-on-error: Expected all 10000 records to be processed, got %d", job.TotalRecords)
	}

	// Verify batch inserts were called multiple times
	if h.userRepo.BatchInsertCalls < 2 {
		t.Errorf("Expected multiple batch inserts, got %d calls", h.userRepo.BatchInsertCalls)
	}
}

func TestProcessImport_UsersCSV_SpecificErrors(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "users_huge.csv")

	job := createTestJob(h, "users", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	storedErrors := h.jobRepo.Errors[job.ID]

	// Line 2 (first data row): missing ID and invalid email "foo@bar" and invalid role "manager"
	hasLine2IDError := false
	hasLine2RoleError := false
	for _, e := range storedErrors {
		if e.Line == 2 && e.Field == "id" {
			hasLine2IDError = true
		}
		if e.Line == 2 && e.Field == "role" {
			hasLine2RoleError = true
		}
	}
	if !hasLine2IDError {
		t.Error("Expected validation error for missing ID on line 2")
	}
	if !hasLine2RoleError {
		t.Error("Expected validation error for invalid role 'manager' on line 2")
	}

	// Line 4: invalid role "reader"
	hasLine4RoleError := false
	for _, e := range storedErrors {
		if e.Line == 4 && e.Field == "role" {
			hasLine4RoleError = true
		}
	}
	if !hasLine4RoleError {
		t.Error("Expected validation error for invalid role 'reader' on line 4")
	}
}

func TestProcessImport_UsersCSV_DuplicateEmails(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "users_huge.csv")

	job := createTestJob(h, "users", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	storedErrors := h.jobRepo.Errors[job.ID]
	duplicateCount := 0
	for _, e := range storedErrors {
		if e.Field == "email" && e.Message == "duplicate email" {
			duplicateCount++
		}
	}
	if duplicateCount == 0 {
		t.Error("Expected duplicate email validation errors to be detected")
	}
	t.Logf("Found %d duplicate email errors", duplicateCount)
}

func TestProcessImport_UsersCSV_EmptyFile(t *testing.T) {
	h := newTestHarness(t)

	tmpFile, err := os.CreateTemp("", "test_empty_*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("id,email,name,role,active,created_at,updated_at\n")
	tmpFile.Close()

	job := createTestJob(h, "users", tmpFile.Name())

	err = h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport should handle empty file: %v", err)
	}

	if job.TotalRecords != 0 {
		t.Errorf("Empty file should have 0 total records, got %d", job.TotalRecords)
	}
	if job.Status != models.JobStatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", job.Status)
	}
}

func TestProcessImport_UsersCSV_BatchInsertError(t *testing.T) {
	h := newTestHarness(t)

	tmpFile, err := os.CreateTemp("", "test_batch_err_*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("id,email,name,role,active,created_at,updated_at\n")
	tmpFile.WriteString("550e8400-e29b-41d4-a716-446655440000,test@example.com,Test User,admin,true,2024-01-01T00:00:00Z,2024-01-01T00:00:00Z\n")
	tmpFile.Close()

	// Make BatchInsert return an error
	h.userRepo.BatchInsertFunc = func(ctx context.Context, users []*models.User) (int, error) {
		return 0, context.DeadlineExceeded
	}

	job := createTestJob(h, "users", tmpFile.Name())

	err = h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport should not return error on batch failure: %v", err)
	}

	// The record should be counted as failed due to batch insert error
	if job.FailedCount != 1 {
		t.Errorf("Expected 1 failed record from batch insert error, got %d", job.FailedCount)
	}
	if job.SuccessfulCount != 0 {
		t.Errorf("Expected 0 successful records, got %d", job.SuccessfulCount)
	}
}

// --- Articles NDJSON Integration Tests ---

func TestProcessImport_ArticlesNDJSON_HugeFile(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "articles_huge.ndjson")

	job := createTestJob(h, "articles", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	// File has 15000 non-empty lines
	if job.TotalRecords != 15000 {
		t.Errorf("Expected 15000 total records, got %d", job.TotalRecords)
	}

	// Verify some records failed
	if job.FailedCount == 0 {
		t.Error("Expected some failed records (invalid slugs, duplicate slugs, missing IDs, drafts with published_at)")
	}

	// Verify successful + failed = total
	if job.SuccessfulCount+job.FailedCount != job.TotalRecords {
		t.Errorf("SuccessfulCount(%d) + FailedCount(%d) = %d, but TotalRecords = %d",
			job.SuccessfulCount, job.FailedCount, job.SuccessfulCount+job.FailedCount, job.TotalRecords)
	}

	// Verify ProcessedCount = TotalRecords
	if job.ProcessedCount != job.TotalRecords {
		t.Errorf("ProcessedCount(%d) should equal TotalRecords(%d)", job.ProcessedCount, job.TotalRecords)
	}

	// Verify records were inserted
	if len(h.articleRepo.Articles) == 0 {
		t.Error("Expected some articles to be inserted into repository")
	}

	// Verify validation errors
	storedErrors := h.jobRepo.Errors[job.ID]
	if len(storedErrors) == 0 {
		t.Error("Expected validation errors to be stored")
	}

	errorFields := make(map[string]bool)
	errorMessages := make(map[string]bool)
	for _, e := range storedErrors {
		errorFields[e.Field] = true
		errorMessages[e.Message] = true
	}

	if !errorFields["id"] {
		t.Error("Expected errors for missing article IDs")
	}
	if !errorFields["slug"] {
		t.Error("Expected errors for invalid/duplicate slugs")
	}
	if !errorMessages["duplicate slug"] {
		t.Error("Expected 'duplicate slug' error message")
	}

	t.Logf("Articles NDJSON: total=%d, successful=%d, failed=%d, errors=%d, batches=%d, duration=%dms, rows/sec=%.0f",
		job.TotalRecords, job.SuccessfulCount, job.FailedCount, len(storedErrors),
		h.articleRepo.BatchInsertCalls, job.DurationMs, job.RowsPerSec)
}

func TestProcessImport_ArticlesNDJSON_DraftWithPublishedAt(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "articles_huge.ndjson")

	job := createTestJob(h, "articles", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	storedErrors := h.jobRepo.Errors[job.ID]
	draftPublishedErrors := 0
	for _, e := range storedErrors {
		if e.Field == "published_at" && e.Message == "draft articles must not have published_at" {
			draftPublishedErrors++
		}
	}
	if draftPublishedErrors == 0 {
		t.Error("Expected validation errors for draft articles with published_at set")
	}
	t.Logf("Found %d 'draft with published_at' errors", draftPublishedErrors)
}

func TestProcessImport_ArticlesNDJSON_MalformedJSON(t *testing.T) {
	h := newTestHarness(t)

	tmpFile, err := os.CreateTemp("", "test_malformed_*.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("{\"id\":\"550e8400-e29b-41d4-a716-446655440000\",\"slug\":\"valid-slug\",\"title\":\"Valid\",\"body\":\"Valid body\",\"author_id\":\"550e8400-e29b-41d4-a716-446655440001\",\"status\":\"draft\"}\n")
	tmpFile.WriteString("{broken json here\n")
	tmpFile.WriteString("{\"id\":\"550e8400-e29b-41d4-a716-446655440002\",\"slug\":\"another-valid\",\"title\":\"Valid2\",\"body\":\"Valid body2\",\"author_id\":\"550e8400-e29b-41d4-a716-446655440001\",\"status\":\"draft\"}\n")
	tmpFile.Close()

	job := createTestJob(h, "articles", tmpFile.Name())

	err = h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("Should handle malformed JSON without fatal error: %v", err)
	}

	if job.TotalRecords != 3 {
		t.Errorf("Expected 3 total records, got %d", job.TotalRecords)
	}
	if job.FailedCount != 1 {
		t.Errorf("Expected 1 failed record (malformed JSON), got %d", job.FailedCount)
	}

	storedErrors := h.jobRepo.Errors[job.ID]
	hasJSONError := false
	for _, e := range storedErrors {
		if e.Field == "json" {
			hasJSONError = true
		}
	}
	if !hasJSONError {
		t.Error("Expected a JSON parse error in validation errors")
	}
}

// --- Comments NDJSON Integration Tests ---

func TestProcessImport_CommentsNDJSON_HugeFile(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "comments_huge.ndjson")

	job := createTestJob(h, "comments", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	// File has 20000 non-empty lines
	if job.TotalRecords != 20000 {
		t.Errorf("Expected 20000 total records, got %d", job.TotalRecords)
	}

	// Verify some records failed
	if job.FailedCount == 0 {
		t.Error("Expected some failed records (missing IDs, invalid FKs, missing bodies)")
	}

	// Verify successful + failed = total
	if job.SuccessfulCount+job.FailedCount != job.TotalRecords {
		t.Errorf("SuccessfulCount(%d) + FailedCount(%d) = %d, but TotalRecords = %d",
			job.SuccessfulCount, job.FailedCount, job.SuccessfulCount+job.FailedCount, job.TotalRecords)
	}

	// Verify ProcessedCount = TotalRecords
	if job.ProcessedCount != job.TotalRecords {
		t.Errorf("ProcessedCount(%d) should equal TotalRecords(%d)", job.ProcessedCount, job.TotalRecords)
	}

	// Verify records were inserted
	if len(h.commentRepo.Comments) == 0 {
		t.Error("Expected some comments to be inserted into repository")
	}

	// Verify validation errors
	storedErrors := h.jobRepo.Errors[job.ID]
	if len(storedErrors) == 0 {
		t.Error("Expected validation errors to be stored")
	}

	errorFields := make(map[string]bool)
	for _, e := range storedErrors {
		errorFields[e.Field] = true
	}

	if !errorFields["id"] {
		t.Error("Expected errors for missing comment IDs")
	}
	if !errorFields["article_id"] {
		t.Error("Expected errors for invalid article_id format")
	}
	if !errorFields["body"] {
		t.Error("Expected errors for missing comment bodies")
	}

	t.Logf("Comments NDJSON: total=%d, successful=%d, failed=%d, errors=%d, batches=%d, duration=%dms, rows/sec=%.0f",
		job.TotalRecords, job.SuccessfulCount, job.FailedCount, len(storedErrors),
		h.commentRepo.BatchInsertCalls, job.DurationMs, job.RowsPerSec)
}

func TestProcessImport_CommentsNDJSON_InvalidForeignKeys(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "comments_huge.ndjson")

	job := createTestJob(h, "comments", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	storedErrors := h.jobRepo.Errors[job.ID]
	invalidArticleIDCount := 0
	for _, e := range storedErrors {
		if e.Field == "article_id" {
			invalidArticleIDCount++
		}
	}
	if invalidArticleIDCount == 0 {
		t.Error("Expected validation errors for invalid article_id foreign keys")
	}
	t.Logf("Found %d invalid article_id errors", invalidArticleIDCount)
}

func TestProcessImport_CommentsNDJSON_MissingBody(t *testing.T) {
	h := newTestHarness(t)
	filePath := testdataPath(t, "comments_huge.ndjson")

	job := createTestJob(h, "comments", filePath)

	err := h.services.Import.ProcessImport(context.Background(), job)
	if err != nil {
		t.Fatalf("ProcessImport returned error: %v", err)
	}

	storedErrors := h.jobRepo.Errors[job.ID]
	missingBodyCount := 0
	for _, e := range storedErrors {
		if e.Field == "body" && e.Message == "body is required" {
			missingBodyCount++
		}
	}
	if missingBodyCount == 0 {
		t.Error("Expected validation errors for missing comment bodies")
	}
	t.Logf("Found %d missing body errors", missingBodyCount)
}

// --- Benchmark ---

func BenchmarkProcessImport_UsersCSV(b *testing.B) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("cannot determine test file path")
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
	filePath := filepath.Join(projectRoot, "testdata", "users_huge.csv")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		b.Skipf("testdata file not found: %s", filePath)
	}

	for i := 0; i < b.N; i++ {
		userRepo := mocks.NewMockUserRepository()
		articleRepo := mocks.NewMockArticleRepository()
		commentRepo := mocks.NewMockCommentRepository()
		jobRepo := mocks.NewMockJobRepository()

		repos := &repository.Repositories{
			User:    userRepo,
			Article: articleRepo,
			Comment: commentRepo,
			Job:     jobRepo,
		}

		cfg := &config.Config{
			Import: config.ImportConfig{
				BatchSize:     1000,
				MaxUploadSize: 500 * 1024 * 1024,
				UploadDir:     os.TempDir(),
			},
		}

		log := zerolog.Nop()
		services := service.NewServices(repos, cfg, log)

		now := time.Now()
		job := &models.Job{
			ID:        "bench-job",
			Type:      models.JobTypeImport,
			Resource:  "users",
			Status:    models.JobStatusPending,
			FilePath:  filePath,
			StartedAt: &now,
			CreatedAt: now,
		}
		jobRepo.Create(context.Background(), job)
		services.Import.ProcessImport(context.Background(), job)
	}
}

func BenchmarkProcessImport_ArticlesNDJSON(b *testing.B) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("cannot determine test file path")
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
	filePath := filepath.Join(projectRoot, "testdata", "articles_huge.ndjson")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		b.Skipf("testdata file not found: %s", filePath)
	}

	for i := 0; i < b.N; i++ {
		userRepo := mocks.NewMockUserRepository()
		articleRepo := mocks.NewMockArticleRepository()
		commentRepo := mocks.NewMockCommentRepository()
		jobRepo := mocks.NewMockJobRepository()

		repos := &repository.Repositories{
			User:    userRepo,
			Article: articleRepo,
			Comment: commentRepo,
			Job:     jobRepo,
		}

		cfg := &config.Config{
			Import: config.ImportConfig{
				BatchSize:     1000,
				MaxUploadSize: 500 * 1024 * 1024,
				UploadDir:     os.TempDir(),
			},
		}

		log := zerolog.Nop()
		services := service.NewServices(repos, cfg, log)

		now := time.Now()
		job := &models.Job{
			ID:        "bench-job",
			Type:      models.JobTypeImport,
			Resource:  "articles",
			Status:    models.JobStatusPending,
			FilePath:  filePath,
			StartedAt: &now,
			CreatedAt: now,
		}
		jobRepo.Create(context.Background(), job)
		services.Import.ProcessImport(context.Background(), job)
	}
}
