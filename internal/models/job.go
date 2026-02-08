package models

import (
	"time"
)

// JobStatus represents the status of an import/export job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
)

// JobType represents the type of job
type JobType string

const (
	JobTypeImport JobType = "import"
	JobTypeExport JobType = "export"
)

// Job represents an import or export job
type Job struct {
	ID              string     `json:"job_id" db:"id"`
	Type            JobType    `json:"type" db:"type"`
	Resource        string     `json:"resource" db:"resource"`
	Status          JobStatus  `json:"status" db:"status"`
	IdempotencyKey  string     `json:"idempotency_key,omitempty" db:"idempotency_key"`
	TotalRecords    int        `json:"total_records" db:"total_records"`
	ProcessedCount  int        `json:"processed" db:"processed_count"`
	SuccessfulCount int        `json:"successful" db:"successful_count"`
	FailedCount     int        `json:"failed" db:"failed_count"`
	DurationMs      int64      `json:"duration_ms,omitempty" db:"duration_ms"`
	RowsPerSec      float64    `json:"rows_per_sec,omitempty" db:"rows_per_sec"`
	FilePath        string     `json:"-" db:"file_path"`
	DownloadURL     string     `json:"download_url,omitempty" db:"download_url"`
	ErrorReportPath string     `json:"-" db:"error_report_path"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty" db:"completed_at"`
}

// ValidationError represents a single validation error
type ValidationError struct {
	Line    int         `json:"line"`
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// JobResponse is the API response for job status
type JobResponse struct {
	Job
	Errors      []ValidationError `json:"errors,omitempty"`
	ErrorCount  int               `json:"error_count,omitempty"`
	ErrorReport string            `json:"error_report_url,omitempty"`
}

// ImportRequest represents an import job request
type ImportRequest struct {
	Resource       string `json:"resource" form:"resource"` // users, articles, comments
	FileURL        string `json:"file_url,omitempty"`       // Remote file URL
	IdempotencyKey string `json:"-"`                        // From header
}

// ExportRequest represents an export job request
type ExportRequest struct {
	Resource string            `json:"resource" form:"resource"` // users, articles, comments
	Format   string            `json:"format" form:"format"`     // json, ndjson, csv
	Filters  map[string]string `json:"filters,omitempty"`        // Optional filters
	Fields   []string          `json:"fields,omitempty"`         // Optional field selection
}
