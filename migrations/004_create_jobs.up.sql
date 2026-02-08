-- Import/Export Jobs table
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(50) NOT NULL CHECK (type IN ('import', 'export')),
    resource VARCHAR(50) NOT NULL CHECK (resource IN ('users', 'articles', 'comments')),
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    idempotency_key VARCHAR(255) UNIQUE,
    
    -- Counters
    total_records INTEGER DEFAULT 0,
    processed_count INTEGER DEFAULT 0,
    successful_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    
    -- Metrics
    duration_ms BIGINT DEFAULT 0,
    rows_per_sec NUMERIC(10, 2) DEFAULT 0,
    
    -- File paths
    file_path TEXT,
    download_url TEXT,
    error_report_path TEXT,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for jobs
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);
CREATE INDEX IF NOT EXISTS idx_jobs_resource ON jobs(resource);
-- Note: idempotency_key already has a UNIQUE constraint which creates an implicit btree index
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);

-- Partial index for pending jobs (efficient for job processor)
CREATE INDEX IF NOT EXISTS idx_jobs_pending ON jobs(created_at) WHERE status = 'pending';
