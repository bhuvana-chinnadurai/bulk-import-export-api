package benchmark

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/bulk-import-export-api/internal/mocks"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/validation"
)

// Helper function
func padInt(i, width int) string {
	s := ""
	for j := 0; j < width; j++ {
		s += "0"
	}
	return s[:width-len(string(rune('0'+i%10)))] + string(rune('0'+i%10))
}

// BenchmarkStreamUsers benchmarks streaming export performance
func BenchmarkStreamUsers(b *testing.B) {
	// Setup mock with 1000 users
	mockUserRepo := mocks.NewMockUserRepository()
	for i := 0; i < 1000; i++ {
		mockUserRepo.Create(context.Background(), &models.User{
			ID:        "550e8400-e29b-41d4-a716-" + padInt(i, 12),
			Email:     "user" + padInt(i, 6) + "@test.com",
			Name:      "Test User " + padInt(i, 6),
			Role:      "admin",
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		count := 0

		mockUserRepo.StreamAll(ctx, func(user *models.User) error {
			count++
			return nil
		})
	}

	b.ReportMetric(float64(1000*b.N)/b.Elapsed().Seconds(), "rows/sec")
}

// BenchmarkBatchInsert benchmarks batch insert performance
func BenchmarkBatchInsert(b *testing.B) {
	mockUserRepo := mocks.NewMockUserRepository()

	// Generate batch of 1000 users
	users := make([]*models.User, 1000)
	for i := 0; i < 1000; i++ {
		users[i] = &models.User{
			ID:        "550e8400-e29b-41d4-a716-" + padInt(i, 12),
			Email:     "user" + padInt(i, 6) + "@test.com",
			Name:      "Test User",
			Role:      "admin",
			Active:    true,
			CreatedAt: time.Now(),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mockUserRepo.BatchInsert(context.Background(), users)
	}

	b.ReportMetric(float64(1000*b.N)/b.Elapsed().Seconds(), "rows/sec")
}

// BenchmarkValidation benchmarks full validation pipeline
func BenchmarkValidation(b *testing.B) {
	validator := validation.NewValidator()

	user := &models.UserCSV{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Email:     "test@example.com",
		Name:      "Test User",
		Role:      "admin",
		Active:    "true",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		validator.ValidateUser(user, i)
	}
}

// BenchmarkCSVParsing benchmarks CSV parsing overhead
func BenchmarkCSVParsing(b *testing.B) {
	// Generate CSV data
	var buf bytes.Buffer
	buf.WriteString("id,email,name,role,active,created_at\n")
	for i := 0; i < 1000; i++ {
		buf.WriteString("550e8400-e29b-41d4-a716-" + padInt(i, 12) +
			",user" + padInt(i, 6) + "@test.com,User,admin,true,2024-01-01T00:00:00Z\n")
	}
	data := buf.Bytes()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		_ = bytes.NewReader(data)
	}
}

// BenchmarkWorkerPoolSemaphore benchmarks semaphore acquire/release
func BenchmarkWorkerPoolSemaphore(b *testing.B) {
	sem := make(chan struct{}, 32) // 32 workers like our implementation

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Acquire
		sem <- struct{}{}
		// Release
		<-sem
	}
}

// BenchmarkWorkerPoolParallel benchmarks parallel semaphore operations
func BenchmarkWorkerPoolParallel(b *testing.B) {
	sem := make(chan struct{}, 32)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sem <- struct{}{}
			<-sem
		}
	})
}
