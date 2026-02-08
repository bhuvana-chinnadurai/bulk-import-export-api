package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/bulk-import-export-api/internal/database"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/lib/pq"
)

// jobRepo is the concrete implementation of JobRepository
type jobRepo struct {
	db *database.DB
}

// NewJobRepo creates a new job repository
func NewJobRepo(db *database.DB) JobRepository {
	return &jobRepo{db: db}
}

// Create inserts a new job
func (r *jobRepo) Create(ctx context.Context, job *models.Job) error {
	query := `
		INSERT INTO jobs (id, type, resource, status, idempotency_key, total_records, 
			processed_count, successful_count, failed_count, file_path, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.ExecContext(ctx, query,
		job.ID, job.Type, job.Resource, job.Status, nullString(job.IdempotencyKey),
		job.TotalRecords, job.ProcessedCount, job.SuccessfulCount, job.FailedCount,
		nullString(job.FilePath), job.CreatedAt,
	)
	return err
}

// Update updates job status and counters
func (r *jobRepo) Update(ctx context.Context, job *models.Job) error {
	query := `
		UPDATE jobs SET 
			status = $1, total_records = $2, processed_count = $3, successful_count = $4, 
			failed_count = $5, duration_ms = $6, rows_per_sec = $7, download_url = $8,
			error_report_path = $9, started_at = $10, completed_at = $11
		WHERE id = $12
	`
	_, err := r.db.ExecContext(ctx, query,
		job.Status, job.TotalRecords, job.ProcessedCount, job.SuccessfulCount,
		job.FailedCount, job.DurationMs, job.RowsPerSec, nullString(job.DownloadURL),
		nullString(job.ErrorReportPath), job.StartedAt, job.CompletedAt, job.ID,
	)
	return err
}

// GetByID retrieves a job by ID
func (r *jobRepo) GetByID(ctx context.Context, id string) (*models.Job, error) {
	query := `
		SELECT id, type, resource, status, idempotency_key, total_records, processed_count, 
			successful_count, failed_count, duration_ms, rows_per_sec, file_path, download_url,
			error_report_path, created_at, started_at, completed_at 
		FROM jobs WHERE id = $1
	`

	var job models.Job
	var idempotencyKey, filePath, downloadURL, errorReportPath sql.NullString
	var startedAt, completedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID, &job.Type, &job.Resource, &job.Status, &idempotencyKey,
		&job.TotalRecords, &job.ProcessedCount, &job.SuccessfulCount, &job.FailedCount,
		&job.DurationMs, &job.RowsPerSec, &filePath, &downloadURL, &errorReportPath,
		&job.CreatedAt, &startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	job.IdempotencyKey = idempotencyKey.String
	job.FilePath = filePath.String
	job.DownloadURL = downloadURL.String
	job.ErrorReportPath = errorReportPath.String
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return &job, nil
}

// GetByIdempotencyKey retrieves a job by idempotency key
func (r *jobRepo) GetByIdempotencyKey(ctx context.Context, key string) (*models.Job, error) {
	query := `
		SELECT id, type, resource, status, idempotency_key, total_records, processed_count, 
			successful_count, failed_count, duration_ms, rows_per_sec, file_path, download_url,
			error_report_path, created_at, started_at, completed_at 
		FROM jobs WHERE idempotency_key = $1
	`

	var job models.Job
	var idempotencyKey, filePath, downloadURL, errorReportPath sql.NullString
	var startedAt, completedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&job.ID, &job.Type, &job.Resource, &job.Status, &idempotencyKey,
		&job.TotalRecords, &job.ProcessedCount, &job.SuccessfulCount, &job.FailedCount,
		&job.DurationMs, &job.RowsPerSec, &filePath, &downloadURL, &errorReportPath,
		&job.CreatedAt, &startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	job.IdempotencyKey = idempotencyKey.String
	job.FilePath = filePath.String
	job.DownloadURL = downloadURL.String
	job.ErrorReportPath = errorReportPath.String
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return &job, nil
}

// GetPendingJobs retrieves all pending jobs
func (r *jobRepo) GetPendingJobs(ctx context.Context) ([]*models.Job, error) {
	query := `
		SELECT id, type, resource, file_path, created_at 
		FROM jobs WHERE status = 'pending' 
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var filePath sql.NullString
		err := rows.Scan(&job.ID, &job.Type, &job.Resource, &filePath, &job.CreatedAt)
		if err != nil {
			continue
		}
		job.FilePath = filePath.String
		job.Status = models.JobStatusPending
		jobs = append(jobs, &job)
	}

	return jobs, rows.Err()
}

// MarkJobAsProcessing atomically marks a pending job as processing
func (r *jobRepo) MarkJobAsProcessing(ctx context.Context, jobID string) (bool, error) {
	query := `
		UPDATE jobs SET status = 'processing', started_at = $1
		WHERE id = $2 AND status = 'pending'
	`
	result, err := r.db.ExecContext(ctx, query, time.Now(), jobID)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

// AddError adds a validation error to the job
func (r *jobRepo) AddError(ctx context.Context, jobID string, err *models.ValidationError) error {
	query := `INSERT INTO job_errors (job_id, line_number, field, message, value) VALUES ($1, $2, $3, $4, $5)`
	valueStr := ""
	if err.Value != nil {
		switch v := err.Value.(type) {
		case string:
			valueStr = v
		}
	}
	_, dbErr := r.db.ExecContext(ctx, query, jobID, err.Line, err.Field, err.Message, valueStr)
	return dbErr
}

// AddErrors adds multiple validation errors using COPY protocol for efficiency
// With 1M records at 10% error rate, this inserts 100K error rows — COPY is
// ~10x faster than individual INSERTs in a prepared statement loop.
func (r *jobRepo) AddErrors(ctx context.Context, jobID string, errors []models.ValidationError) error {
	if len(errors) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("job_errors",
		"job_id", "line_number", "field", "message", "value",
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range errors {
		valueStr := ""
		if e.Value != nil {
			switch v := e.Value.(type) {
			case string:
				valueStr = v
			}
		}
		stmt.ExecContext(ctx, jobID, e.Line, e.Field, e.Message, valueStr)
	}

	// Flush the COPY buffer
	if _, err := stmt.ExecContext(ctx); err != nil {
		return err
	}

	return tx.Commit()
}

// GetErrors retrieves validation errors for a job
func (r *jobRepo) GetErrors(ctx context.Context, jobID string, limit int) ([]models.ValidationError, error) {
	query := `SELECT line_number, field, message, value FROM job_errors WHERE job_id = $1 ORDER BY line_number`
	if limit > 0 {
		query += " LIMIT $2"
	}

	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = r.db.QueryContext(ctx, query, jobID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, query, jobID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errors []models.ValidationError
	for rows.Next() {
		var e models.ValidationError
		var value sql.NullString
		err := rows.Scan(&e.Line, &e.Field, &e.Message, &value)
		if err != nil {
			continue
		}
		if value.Valid {
			e.Value = value.String
		}
		errors = append(errors, e)
	}

	return errors, rows.Err()
}

// helper to convert empty string to NULL
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
