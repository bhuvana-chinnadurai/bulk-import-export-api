package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bulk-import-export-api/internal/mocks"
	"github.com/bulk-import-export-api/internal/models"
)

func TestMockUserRepository_BatchInsert(t *testing.T) {
	repo := mocks.NewMockUserRepository()
	ctx := context.Background()

	users := []*models.User{
		{ID: "user-1", Email: "user1@test.com", Name: "User 1", Role: "admin", Active: true, CreatedAt: time.Now()},
		{ID: "user-2", Email: "user2@test.com", Name: "User 2", Role: "editor", Active: true, CreatedAt: time.Now()},
		{ID: "user-3", Email: "user3@test.com", Name: "User 3", Role: "viewer", Active: false, CreatedAt: time.Now()},
	}

	inserted, err := repo.BatchInsert(ctx, users)
	if err != nil {
		t.Fatalf("BatchInsert failed: %v", err)
	}

	if inserted != 3 {
		t.Errorf("Expected 3 inserted, got %d", inserted)
	}

	if len(repo.Users) != 3 {
		t.Errorf("Expected 3 users in repo, got %d", len(repo.Users))
	}

	// Verify users are retrievable
	for _, u := range users {
		stored, err := repo.GetByID(ctx, u.ID)
		if err != nil {
			t.Errorf("GetByID failed: %v", err)
		}
		if stored == nil {
			t.Errorf("User %s not found", u.ID)
		}
	}
}

func TestMockUserRepository_DuplicateEmail(t *testing.T) {
	repo := mocks.NewMockUserRepository()
	ctx := context.Background()

	// Insert first user
	user1 := &models.User{ID: "user-1", Email: "duplicate@test.com", Name: "User 1", Role: "admin", Active: true}
	err := repo.Create(ctx, user1)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Check email exists
	exists, err := repo.EmailExists(ctx, "duplicate@test.com")
	if err != nil {
		t.Fatalf("EmailExists failed: %v", err)
	}
	if !exists {
		t.Error("Email should exist")
	}

	// Check non-existent email
	exists, err = repo.EmailExists(ctx, "nonexistent@test.com")
	if err != nil {
		t.Fatalf("EmailExists failed: %v", err)
	}
	if exists {
		t.Error("Email should not exist")
	}
}

func TestMockUserRepository_Count(t *testing.T) {
	repo := mocks.NewMockUserRepository()
	ctx := context.Background()

	// Initially empty
	count, _ := repo.Count(ctx)
	if count != 0 {
		t.Errorf("Expected 0, got %d", count)
	}

	// Add users
	for i := 0; i < 5; i++ {
		repo.Create(ctx, &models.User{
			ID:    fmt.Sprintf("user-%d", i),
			Email: fmt.Sprintf("user%d@test.com", i),
		})
	}

	count, _ = repo.Count(ctx)
	if count != 5 {
		t.Errorf("Expected 5, got %d", count)
	}
}

func TestMockJobRepository_PendingJobs(t *testing.T) {
	repo := mocks.NewMockJobRepository()
	ctx := context.Background()

	// Create jobs with different statuses
	jobs := []*models.Job{
		{ID: "job-1", Status: models.JobStatusPending, Resource: "users"},
		{ID: "job-2", Status: models.JobStatusProcessing, Resource: "articles"},
		{ID: "job-3", Status: models.JobStatusPending, Resource: "comments"},
		{ID: "job-4", Status: models.JobStatusCompleted, Resource: "users"},
	}

	for _, job := range jobs {
		repo.Create(ctx, job)
	}

	// Get pending jobs
	pending, err := repo.GetPendingJobs(ctx)
	if err != nil {
		t.Fatalf("GetPendingJobs failed: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("Expected 2 pending jobs, got %d", len(pending))
	}
}

func TestMockJobRepository_MarkAsProcessing(t *testing.T) {
	repo := mocks.NewMockJobRepository()
	ctx := context.Background()

	job := &models.Job{ID: "job-1", Status: models.JobStatusPending, Resource: "users"}
	repo.Create(ctx, job)

	// Mark as processing
	marked, err := repo.MarkJobAsProcessing(ctx, "job-1")
	if err != nil {
		t.Fatalf("MarkJobAsProcessing failed: %v", err)
	}
	if !marked {
		t.Error("Job should be marked as processing")
	}

	// Try to mark again (should fail - already processing)
	marked, _ = repo.MarkJobAsProcessing(ctx, "job-1")
	if marked {
		t.Error("Job should not be marked again")
	}
}

func TestMockJobRepository_ValidationErrors(t *testing.T) {
	repo := mocks.NewMockJobRepository()
	ctx := context.Background()

	job := &models.Job{ID: "job-1", Status: models.JobStatusPending, Resource: "users"}
	repo.Create(ctx, job)

	// Add errors
	errors := []models.ValidationError{
		{Line: 1, Field: "email", Message: "invalid email", Value: "not-an-email"},
		{Line: 2, Field: "role", Message: "invalid role", Value: "superuser"},
		{Line: 5, Field: "id", Message: "missing id", Value: nil},
	}
	repo.AddErrors(ctx, "job-1", errors)

	// Retrieve errors
	retrieved, err := repo.GetErrors(ctx, "job-1", 0)
	if err != nil {
		t.Fatalf("GetErrors failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(retrieved))
	}

	// Test limit
	retrieved, _ = repo.GetErrors(ctx, "job-1", 2)
	if len(retrieved) != 2 {
		t.Errorf("Expected 2 errors with limit, got %d", len(retrieved))
	}
}

func TestMockJobRepository_IdempotencyKey(t *testing.T) {
	repo := mocks.NewMockJobRepository()
	ctx := context.Background()

	job := &models.Job{
		ID:             "job-1",
		Status:         models.JobStatusPending,
		Resource:       "users",
		IdempotencyKey: "unique-key-123",
	}
	repo.Create(ctx, job)

	// Retrieve by idempotency key
	retrieved, err := repo.GetByIdempotencyKey(ctx, "unique-key-123")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Job should be found by idempotency key")
	}
	if retrieved.ID != "job-1" {
		t.Errorf("Expected job-1, got %s", retrieved.ID)
	}

	// Non-existent key
	retrieved, _ = repo.GetByIdempotencyKey(ctx, "non-existent")
	if retrieved != nil {
		t.Error("Should not find job with non-existent key")
	}
}
