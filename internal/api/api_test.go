package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bulk-import-export-api/internal/api"
	"github.com/bulk-import-export-api/internal/config"
	"github.com/bulk-import-export-api/internal/mocks"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func setupTestRouter() (*gin.Engine, *mocks.MockImportService, *mocks.MockExportService, *mocks.MockJobService) {
	gin.SetMode(gin.TestMode)

	mockImport := mocks.NewMockImportService()
	mockExport := mocks.NewMockExportService()
	mockJob := mocks.NewMockJobService()

	services := &service.Services{
		Import: mockImport,
		Export: mockExport,
		Job:    mockJob,
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Port: "8080"},
		Import: config.ImportConfig{
			BatchSize:     1000,
			MaxUploadSize: 500 * 1024 * 1024,
			UploadDir:     "/tmp/test-uploads",
		},
	}

	log := zerolog.Nop()
	router := api.NewRouter(services, cfg, log)

	return router, mockImport, mockExport, mockJob
}

func TestHealthEndpoint(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}
	if response["service"] != "bulk-import-export-api" {
		t.Errorf("Expected service name, got %v", response["service"])
	}
}

func TestMetricsEndpoint(t *testing.T) {
	router, _, mockExport, _ := setupTestRouter()
	mockExport.Counts["users"] = 1000
	mockExport.Counts["articles"] = 500
	mockExport.Counts["comments"] = 2000

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	db := response["database"].(map[string]interface{})
	if db["users"].(float64) != 1000 {
		t.Errorf("Expected 1000 users, got %v", db["users"])
	}
}

func TestGetImportStatus(t *testing.T) {
	router, _, _, mockJob := setupTestRouter()

	// Add a test job
	now := time.Now()
	mockJob.Jobs["test-job-123"] = &models.JobResponse{
		Job: models.Job{
			ID:              "test-job-123",
			Type:            models.JobTypeImport,
			Resource:        "users",
			Status:          models.JobStatusCompleted,
			TotalRecords:    1000,
			SuccessfulCount: 950,
			FailedCount:     50,
			DurationMs:      5000,
			RowsPerSec:      200.0,
			CreatedAt:       now,
		},
		ErrorCount: 50,
	}

	req := httptest.NewRequest("GET", "/v1/imports/test-job-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response models.JobResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.Job.ID != "test-job-123" {
		t.Errorf("Expected job ID 'test-job-123', got '%s'", response.Job.ID)
	}
	if response.Job.Status != models.JobStatusCompleted {
		t.Errorf("Expected status completed, got %s", response.Job.Status)
	}
	if response.Job.TotalRecords != 1000 {
		t.Errorf("Expected 1000 total records, got %d", response.Job.TotalRecords)
	}
}

func TestGetImportStatus_NotFound(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/v1/imports/nonexistent-job", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetImportErrors(t *testing.T) {
	router, _, _, mockJob := setupTestRouter()

	// Add job with errors
	mockJob.Errors["job-with-errors"] = []models.ValidationError{
		{Line: 1, Field: "email", Message: "invalid email format", Value: "not-an-email"},
		{Line: 5, Field: "role", Message: "invalid role", Value: "superadmin"},
		{Line: 10, Field: "id", Message: "missing required field", Value: nil},
	}

	req := httptest.NewRequest("GET", "/v1/imports/job-with-errors/errors", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["error_count"].(float64) != 3 {
		t.Errorf("Expected 3 errors, got %v", response["error_count"])
	}

	errors := response["errors"].([]interface{})
	if len(errors) != 3 {
		t.Errorf("Expected 3 error details, got %d", len(errors))
	}
}

func TestGetImportErrors_CSV(t *testing.T) {
	router, _, _, mockJob := setupTestRouter()

	mockJob.Errors["job-with-errors"] = []models.ValidationError{
		{Line: 1, Field: "email", Message: "invalid email", Value: "bad@"},
	}

	req := httptest.NewRequest("GET", "/v1/imports/job-with-errors/errors?format=csv", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("Expected text/csv, got %s", contentType)
	}

	body := w.Body.String()
	if !bytes.Contains(w.Body.Bytes(), []byte("line,field,message,value")) {
		t.Error("CSV should contain header row")
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("email")) {
		t.Errorf("CSV should contain error data, got: %s", body)
	}
}

func TestExportStream_ValidationErrors(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing resource",
			url:            "/v1/exports",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "resource parameter is required",
		},
		{
			name:           "invalid resource",
			url:            "/v1/exports?resource=invalid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "resource must be one of",
		},
		{
			name:           "invalid format",
			url:            "/v1/exports?resource=users&format=xml",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "format must be one of",
		},
		{
			name:           "csv not supported for articles",
			url:            "/v1/exports?resource=articles&format=csv",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "CSV format only supported for users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedError != "" && !bytes.Contains(w.Body.Bytes(), []byte(tt.expectedError)) {
				t.Errorf("Expected error '%s' in response, got: %s", tt.expectedError, w.Body.String())
			}
		})
	}
}

func TestImportValidation(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	tests := []struct {
		name           string
		resource       string
		expectedStatus int
	}{
		{"missing resource", "", http.StatusBadRequest},
		{"invalid resource", "invalid", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			if tt.resource != "" {
				writer.WriteField("resource", tt.resource)
			}
			writer.Close()

			req := httptest.NewRequest("POST", "/v1/imports", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestIdempotencyKey(t *testing.T) {
	router, _, _, mockJob := setupTestRouter()

	// Add existing job with idempotency key
	existingJob := &models.JobResponse{
		Job: models.Job{
			ID:             "existing-job-123",
			Resource:       "users",
			Status:         models.JobStatusCompleted,
			IdempotencyKey: "unique-idempotency-key",
		},
	}
	mockJob.Jobs["existing-job-123"] = existingJob

	// Create a file upload request with same idempotency key
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("resource", "users")
	part, _ := writer.CreateFormFile("file", "test.csv")
	part.Write([]byte("id,email,name,role,active,created_at\n"))
	writer.Close()

	req := httptest.NewRequest("POST", "/v1/imports", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Idempotency-Key", "unique-idempotency-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return existing job, not create new one
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (existing job), got %d", w.Code)
	}
}

func TestCORSHeaders(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	req := httptest.NewRequest("OPTIONS", "/v1/imports", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204 for OPTIONS, got %d", w.Code)
	}

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin '*', got '%s'", allowOrigin)
	}

	allowMethods := w.Header().Get("Access-Control-Allow-Methods")
	if allowMethods == "" {
		t.Error("Expected Access-Control-Allow-Methods header")
	}
}

func TestImportWithWrongFileExtension(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	tests := []struct {
		name           string
		resource       string
		filename       string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "users with ndjson file",
			resource:       "users",
			filename:       "users.ndjson",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "users import requires CSV file",
		},
		{
			name:           "articles with csv file",
			resource:       "articles",
			filename:       "articles.csv",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "articles/comments import requires NDJSON file",
		},
		{
			name:           "comments with csv file",
			resource:       "comments",
			filename:       "comments.csv",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "articles/comments import requires NDJSON file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			writer.WriteField("resource", tt.resource)
			part, _ := writer.CreateFormFile("file", tt.filename)
			part.Write([]byte("test data\n"))
			writer.Close()

			req := httptest.NewRequest("POST", "/v1/imports", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte(tt.expectedError)) {
				t.Errorf("Expected error '%s' in response, got: %s", tt.expectedError, w.Body.String())
			}
		})
	}
}

func TestCreateExport_Validation(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	tests := []struct {
		name           string
		body           string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing resource",
			body:           `{"format":"ndjson"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "resource is required",
		},
		{
			name:           "invalid resource",
			body:           `{"resource":"invalid","format":"ndjson"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "resource must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/exports", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte(tt.expectedError)) {
				t.Errorf("Expected error '%s' in response, got: %s", tt.expectedError, w.Body.String())
			}
		})
	}
}

func TestGetExportStatus_NotFound(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/v1/exports/nonexistent-job", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetImportErrors_EmptyErrors(t *testing.T) {
	router, _, _, mockJob := setupTestRouter()

	// Job exists but has no errors
	mockJob.Errors["job-no-errors"] = []models.ValidationError{}

	req := httptest.NewRequest("GET", "/v1/imports/job-no-errors/errors", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["error_count"].(float64) != 0 {
		t.Errorf("Expected 0 errors, got %v", response["error_count"])
	}
}

// Placeholder for unused imports
var _ context.Context
var _ api.ImportHandler
