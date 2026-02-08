-- Job validation errors table
CREATE TABLE IF NOT EXISTS job_errors (
    id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    line_number INTEGER NOT NULL,
    field VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    value TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for job_errors
CREATE INDEX IF NOT EXISTS idx_job_errors_job_id ON job_errors(job_id);
CREATE INDEX IF NOT EXISTS idx_job_errors_job_line ON job_errors(job_id, line_number);
