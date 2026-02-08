package service

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/repository"
	"github.com/rs/zerolog"
)

// jobService is the concrete implementation of JobService
type jobService struct {
	jobRepo       repository.JobRepository
	importService ImportService
	log           zerolog.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       bool
	mu            sync.Mutex
	// Semaphore: buffered channel to limit concurrent job processing
	// Based on Dave Cheney's recommendation to prevent OOM by limiting goroutines
	sem chan struct{}
}

// newJobService creates a new JobService with worker pool sized for I/O-bound work
func newJobService(jobRepo repository.JobRepository, log zerolog.Logger) *jobService {
	// For I/O-bound work (database/file operations), we can have more workers than CPU cores
	// since most time is spent waiting for I/O, not computing
	// Common formula: NumCPU * 2-10 for I/O-bound, NumCPU for CPU-bound
	maxWorkers := runtime.NumCPU() * 4
	if maxWorkers < 4 {
		maxWorkers = 4 // Minimum 4 workers for I/O-bound work
	}
	if maxWorkers > 32 {
		maxWorkers = 32 // Cap to avoid excessive connections
	}

	log.Info().Int("max_workers", maxWorkers).Msg("Initializing job service worker pool (I/O-bound)")

	return &jobService{
		jobRepo: jobRepo,
		log:     log.With().Str("service", "job").Logger(),
		sem:     make(chan struct{}, maxWorkers), // Semaphore limits concurrent jobs
	}
}

// SetImportService sets the import service for job processing
func (s *jobService) SetImportService(importService ImportService) {
	s.importService = importService
}

// StartProcessor starts the background job processor
func (s *jobService) StartProcessor(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	s.log.Info().Msg("Job processor started")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.log.Info().Msg("Job processor stopping")
			return
		case <-ticker.C:
			s.processPendingJobs()
		}
	}
}

// StopProcessor stops the background job processor
func (s *jobService) StopProcessor() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.wg.Wait()
	s.running = false
	s.log.Info().Msg("Job processor stopped")
}

// processPendingJobs processes all pending jobs
func (s *jobService) processPendingJobs() {
	jobs, err := s.jobRepo.GetPendingJobs(s.ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to get pending jobs")
		return
	}

	for _, job := range jobs {
		// Acquire semaphore slot - blocks if all workers are busy (backpressure)
		// This prevents spawning unlimited goroutines which could cause OOM
		select {
		case s.sem <- struct{}{}:
			// Got a slot, proceed
		case <-s.ctx.Done():
			// Shutdown requested
			return
		}

		// Mark as processing atomically
		marked, err := s.jobRepo.MarkJobAsProcessing(s.ctx, job.ID)
		if err != nil || !marked {
			<-s.sem  // Release slot since we're not processing this job
			continue // Another worker already picked it up
		}

		s.wg.Add(1)
		go func(j *models.Job) {
			defer s.wg.Done()
			defer func() { <-s.sem }() // Release semaphore slot when done

			// Panic recovery - prevents runtime panics from crashing the entire process
			defer func() {
				if r := recover(); r != nil {
					s.log.Error().
						Interface("panic", r).
						Str("job_id", j.ID).
						Msg("Job processing panicked - recovered")
					// Mark job as failed
					j.Status = models.JobStatusFailed
					s.jobRepo.Update(s.ctx, j)
				}
			}()
			s.processJob(j)
		}(job)
	}
}

// processJob processes a single job
func (s *jobService) processJob(job *models.Job) {
	// Check if context is cancelled before processing (Dave Cheney: respect context cancellation)
	select {
	case <-s.ctx.Done():
		s.log.Warn().Str("job_id", job.ID).Msg("Job processing cancelled due to shutdown")
		return
	default:
	}

	s.log.Info().Str("job_id", job.ID).Str("type", string(job.Type)).Msg("Processing job")

	switch job.Type {
	case models.JobTypeImport:
		if s.importService != nil {
			if err := s.importService.ProcessImport(s.ctx, job); err != nil {
				s.log.Error().Err(err).Str("job_id", job.ID).Msg("Import processing failed")
			}
		}
	case models.JobTypeExport:
		// Export jobs are handled synchronously for streaming
		s.log.Warn().Str("job_id", job.ID).Msg("Export job not processed in background")
	}
}

// GetJob retrieves a job by ID with errors
func (s *jobService) GetJob(ctx context.Context, id string) (*models.JobResponse, error) {
	job, err := s.jobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}

	// Get validation errors (limit to first 100)
	errors, err := s.jobRepo.GetErrors(ctx, id, 100)
	if err != nil {
		s.log.Error().Err(err).Str("job_id", id).Msg("Failed to get job errors")
	}

	response := &models.JobResponse{
		Job:        *job,
		Errors:     errors,
		ErrorCount: job.FailedCount,
	}

	// Add error report URL if there are errors
	if job.FailedCount > 0 {
		response.ErrorReport = "/v1/imports/" + job.ID + "/errors"
	}

	return response, nil
}

// GetJobByIdempotencyKey retrieves a job by idempotency key
func (s *jobService) GetJobByIdempotencyKey(ctx context.Context, key string) (*models.Job, error) {
	return s.jobRepo.GetByIdempotencyKey(ctx, key)
}

// GetJobErrors retrieves all validation errors for a job
func (s *jobService) GetJobErrors(ctx context.Context, id string) ([]models.ValidationError, error) {
	return s.jobRepo.GetErrors(ctx, id, 0)
}
