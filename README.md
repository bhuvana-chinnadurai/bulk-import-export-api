# Bulk Import/Export API

A high-performance Go REST API for bulk importing and exporting articles, comments, and users in JSON/NDJSON/CSV formats. Designed to handle large datasets (up to 1M records) efficiently using streaming and asynchronous job processing.

## Features

- **Streaming Processing**: Handles large files with O(1) memory using `csv.Reader` and `bufio.Scanner`
- **Multiple Formats**: Supports JSON, NDJSON, and CSV formats
- **Async Job Processing**: Background worker pool with semaphore-based concurrency control
- **Batch Writes**: PostgreSQL COPY protocol for 1,000-record batch inserts
- **Robust Validation**: Per-record validation with continue-on-error semantics and detailed error reporting
- **Idempotency**: `Idempotency-Key` header prevents duplicate job processing
- **Structured Logging**: zerolog with `rows/sec`, `error_rate_pct`, `duration_ms` per job
- **Context Cancellation**: Long-running imports respect `context.Done()` for graceful shutdown

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         API Layer (Gin)                         │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────┐   │
│  │ POST /imports │  │ GET /exports  │  │ GET /imports/:id  │   │
│  └───────┬───────┘  └───────┬───────┘  └─────────┬─────────┘   │
├──────────┼──────────────────┼────────────────────┼──────────────┤
│          ▼                  ▼                    ▼              │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   Service Layer                          │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │   │
│  │  │   Import    │  │   Export    │  │  Job Manager    │  │   │
│  │  │   Service   │  │   Service   │  │   (Goroutines)  │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
├──────────────────────────────┼──────────────────────────────────┤
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │               Data Layer (PostgreSQL)                    │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │   │
│  │  │    Users    │  │  Articles   │  │    Comments     │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| [gin-gonic/gin](https://github.com/gin-gonic/gin) | v1.9.1 | HTTP framework with middleware, routing, JSON binding |
| [lib/pq](https://github.com/lib/pq) | v1.10.9 | PostgreSQL driver with COPY protocol for batch inserts |
| [rs/zerolog](https://github.com/rs/zerolog) | v1.31.0 | Zero-allocation structured JSON logging |
| [google/uuid](https://github.com/google/uuid) | v1.5.0 | UUID generation and validation |
| [golang-migrate](https://github.com/golang-migrate/migrate) | v4.17.0 | Database schema migrations |

## API Endpoints

### Import Endpoints (Async Jobs)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/imports` | Upload file (multipart). Returns job_id |
| GET | `/v1/imports/:job_id` | Get job status, counters, and validation errors |
| GET | `/v1/imports/:job_id/errors` | Get validation errors (JSON or `?format=csv`) |

**Headers:**
- `Idempotency-Key`: Prevents duplicate processing of the same import

### Export Endpoints (Streaming)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/exports?resource=articles&format=ndjson` | Stream export data directly |
| POST | `/v1/exports` | Create async export job with filters |
| GET | `/v1/exports/:job_id` | Get export job status |

### Health & Metrics

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check endpoint |
| GET | `/metrics` | Database record counts per resource |

## Data Models

### User
```json
{
  "id": "uuid",
  "email": "string (valid, unique)",
  "name": "string",
  "role": "admin | editor | viewer",
  "active": true,
  "created_at": "ISO 8601",
  "updated_at": "ISO 8601"
}
```

### Article
```json
{
  "id": "uuid",
  "slug": "string (unique, kebab-case)",
  "title": "string",
  "body": "string",
  "author_id": "uuid (FK to users)",
  "tags": ["string"],
  "status": "draft | published",
  "published_at": "ISO 8601 (null for drafts)",
  "created_at": "ISO 8601"
}
```

### Comment
```json
{
  "id": "uuid",
  "article_id": "uuid (FK to articles)",
  "user_id": "uuid (FK to users)",
  "body": "string (max 500 words)",
  "created_at": "ISO 8601"
}
```

## Validation Rules

- **Users**: Valid UUID id; valid unique email; role in [admin, editor, viewer]; boolean active; ISO 8601 created_at
- **Articles**: Valid UUID id; unique kebab-case slug; valid author_id FK; draft must NOT have published_at
- **Comments**: Valid UUID id; valid article_id and user_id FKs; body required (max 500 words); ISO 8601 created_at
- **Duplicate detection**: Emails (users) and slugs (articles) checked within each import batch
- **FK validation**: author_id and article_id validated against in-memory cache of existing IDs

## Quick Start

### Prerequisites
- Go 1.21+
- PostgreSQL 14+
- Docker (optional, for containerized setup)

### Installation

```bash
# Clone and enter directory
cd bulk-import-export-api

# Install dependencies
go mod tidy

# Set up PostgreSQL (or use Docker)
export DB_HOST=localhost DB_PORT=5432 DB_USER=postgres DB_PASSWORD=postgres DB_NAME=bulk_import_export

# Run the server
go run cmd/server/main.go
```

### Using Docker

```bash
# Build and run (includes PostgreSQL)
docker-compose up --build

# The API will be available at http://localhost:8080
# Sample data (10 users, 5 articles, 5 comments) is auto-seeded on first startup
# PostgreSQL is exposed on port 5433 (mapped from container's 5432)
```

On first startup, the container automatically seeds the database with sample data via the import API (10 users, 5 articles, 5 comments). Subsequent restarts skip seeding if data already exists. Check `/metrics` to verify:

```bash
curl http://localhost:8080/metrics
# {"database":{"users":10,"articles":5,"comments":5},...}
```

**Connect to PostgreSQL** (after `docker-compose up -d`):
```bash
# psql
psql -h localhost -p 5433 -U postgres -d bulk_import_export

# Or use any GUI client (TablePlus, pgAdmin, DBeaver, DataGrip):
#   Host: localhost, Port: 5433, User: postgres, Password: postgres, DB: bulk_import_export
```

### Postman Collection

Import `postman_collection.json` into Postman to test all endpoints interactively. The collection includes:
- All import/export endpoints with test scripts
- Automatic `job_id` extraction and chaining between requests
- Idempotency key deduplication demo
- Validation error cases (missing resource, wrong file extension, invalid format)
- Pre-configured to use `testdata/` files

### Example Usage

#### Import Users (CSV)
```bash
curl -X POST http://localhost:8080/v1/imports \
  -H "Idempotency-Key: import-users-001" \
  -F "file=@testdata/users_huge.csv" \
  -F "resource=users"
```

#### Import Articles (NDJSON)
```bash
curl -X POST http://localhost:8080/v1/imports \
  -H "Idempotency-Key: import-articles-001" \
  -F "file=@testdata/articles_huge.ndjson" \
  -F "resource=articles"
```

#### Import Comments (NDJSON)
```bash
curl -X POST http://localhost:8080/v1/imports \
  -H "Idempotency-Key: import-comments-001" \
  -F "file=@testdata/comments_huge.ndjson" \
  -F "resource=comments"
```

#### Check Import Job Status
```bash
curl http://localhost:8080/v1/imports/{job_id}
```

Response:
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "resource": "users",
  "total_records": 10000,
  "processed": 10000,
  "successful": 3233,
  "failed": 6767,
  "duration_ms": 14,
  "rows_per_sec": 567056,
  "error_report": "/v1/imports/550e8400.../errors",
  "errors": [
    {
      "line": 2,
      "field": "id",
      "message": "id is required"
    },
    {
      "line": 2,
      "field": "role",
      "message": "invalid role, must be one of: admin, editor, viewer",
      "value": "manager"
    }
  ]
}
```

#### Download Error Report as CSV
```bash
curl "http://localhost:8080/v1/imports/{job_id}/errors?format=csv" -o errors.csv
```

#### Export Articles (NDJSON Streaming)
```bash
curl "http://localhost:8080/v1/exports?resource=articles&format=ndjson" \
  -o articles_export.ndjson
```

#### Export Users (CSV)
```bash
curl "http://localhost:8080/v1/exports?resource=users&format=csv" \
  -o users_export.csv
```

#### Export Comments (JSON)
```bash
curl "http://localhost:8080/v1/exports?resource=comments&format=json" \
  -o comments_export.json
```

## Performance

### Design

| Aspect | Implementation |
|---|---|
| **Target** | Handle up to 1,000,000 records per job |
| **Memory** | O(1) streaming via `csv.Reader` / `bufio.Scanner` - constant memory regardless of file size |
| **Batch writes** | 1,000 records per PostgreSQL COPY transaction |
| **Concurrency** | Semaphore-bounded goroutine worker pool (`NumCPU * 4`, capped at 32) |
| **Export streaming** | NDJSON/JSON/CSV streamed directly to HTTP response with `http.Flusher` every 100 records (target: 5K+ rows/sec) |
| **Context cancellation** | Checked every 10,000 records for graceful shutdown of long-running imports |

### Observability

Each completed import logs structured JSON with:

```json
{
  "job_id": "...",
  "total": 10000,
  "successful": 3233,
  "failed": 6767,
  "error_rate_pct": 67.67,
  "duration_ms": 14,
  "rows_per_sec": 567056,
  "msg": "Import completed"
}
```

### Benchmark Results

Benchmarks run against real testdata files (with mock database layer):

| Resource | Records | Avg Time | Throughput |
|---|---|---|---|
| Users CSV | 10,000 | ~14ms | ~714K rows/sec |
| Articles NDJSON | 15,000 | ~177ms | ~85K rows/sec |

```bash
# Run benchmarks
go test ./internal/service/ -bench=. -benchtime=3s
```

## Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go                          # Application entry point
├── internal/
│   ├── api/
│   │   ├── router.go                        # Route definitions + middleware
│   │   ├── import_handler.go                # Import endpoints
│   │   └── export_handler.go                # Export endpoints
│   ├── config/
│   │   └── config.go                        # Environment-based configuration
│   ├── database/
│   │   └── db.go                            # PostgreSQL connection + migrations
│   ├── models/
│   │   ├── user.go, article.go, comment.go  # Domain models
│   │   └── job.go                           # Job model with status tracking
│   ├── mocks/
│   │   └── repositories.go                  # Mock implementations for testing
│   ├── repository/
│   │   ├── repository.go                    # Repository interfaces
│   │   ├── user_repo.go                     # PostgreSQL COPY batch inserts
│   │   ├── article_repo.go
│   │   ├── comment_repo.go
│   │   └── job_repo.go
│   ├── service/
│   │   ├── import_service.go                # Streaming import with batch processing
│   │   ├── export_service.go                # Streaming export with HTTP flushing
│   │   ├── job_service.go                   # Background worker pool (semaphore)
│   │   ├── services.go                      # Service interfaces + DI wiring
│   │   └── import_integration_test.go       # Integration tests with real testdata
│   └── validation/
│       ├── validator.go                     # Validation rules + duplicate detection
│       ├── validator_test.go                # Unit tests + boundary tests
│       └── validator_integration_test.go    # Integration tests with real testdata
├── testdata/
│   ├── seed_users.csv                       # 10 valid users (auto-seeded on Docker startup)
│   ├── seed_articles.ndjson                 # 5 valid articles (auto-seeded on Docker startup)
│   ├── seed_comments.ndjson                 # 5 valid comments (auto-seeded on Docker startup)
│   ├── users_huge.csv                       # 10,000 users with intentional errors
│   ├── articles_huge.ndjson                 # 15,000 articles with intentional errors
│   └── comments_huge.ndjson                 # 20,000 comments with intentional errors
├── pkg/
│   └── logger/
│       └── logger.go                        # Structured logging setup
├── scripts/
│   ├── entrypoint.sh                      # Docker entrypoint: starts server + seeds in background
│   ├── seed.sh                            # Auto-seeds database via import API on first startup
│   ├── benchmark.sh                       # End-to-end perf test (import + export throughput)
│   └── loadtest.js                        # k6 concurrent HTTP load test (43 req/s, all endpoints)
├── test/
│   └── benchmark/
│       └── benchmark_test.go              # Micro-benchmarks (streaming, batch, validation, semaphore)
├── migrations/                            # PostgreSQL schema migrations
├── Dockerfile
├── docker-compose.yml
├── postman_collection.json                # Postman collection for all endpoints
├── go.mod
└── go.sum
```

## Configuration

Environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `postgres` |
| `DB_PASSWORD` | PostgreSQL password | `postgres` |
| `DB_NAME` | Database name | `bulk_import_export` |
| `DB_SSLMODE` | SSL mode | `disable` |
| `DB_MAX_OPEN_CONNS` | Max open connections | `25` |
| `DB_MAX_IDLE_CONNS` | Max idle connections | `5` |
| `IMPORT_BATCH_SIZE` | Records per batch insert | `1000` |
| `MAX_UPLOAD_SIZE` | Maximum upload file size (bytes) | `524288000` (500MB) |
| `UPLOAD_DIR` | File upload directory | `./data/uploads` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `LOG_FORMAT` | Log format (json, pretty) | `json` |

## Testing

```bash
# Run all unit + integration tests
CGO_ENABLED=0 go test ./... -v -count=1

# Run with coverage
CGO_ENABLED=0 go test ./... -cover -count=1

# Run benchmarks (pipeline throughput against real testdata)
CGO_ENABLED=0 go test ./internal/service/ -bench=. -benchtime=3s

# Run micro-benchmarks (streaming, batch insert, validation)
CGO_ENABLED=0 go test ./test/benchmark/ -bench=. -benchtime=3s
```

### Performance Testing (requires Docker)

```bash
# Start the API with PostgreSQL
docker-compose up -d

# End-to-end benchmark: import + export with configurable record count
./scripts/benchmark.sh              # 10K records (default)
./scripts/benchmark.sh 100000       # 100K records
./scripts/benchmark.sh 1000000      # 1M records (proves assignment target)

# k6 concurrent HTTP load test: 43 req/s across all 6 endpoints for 30s
k6 run scripts/loadtest.js

# Import the provided testdata through the real API
curl -X POST http://localhost:8080/v1/imports \
  -H "Idempotency-Key: test-users" -F "file=@testdata/users_huge.csv" -F "resource=users"
curl -X POST http://localhost:8080/v1/imports \
  -H "Idempotency-Key: test-articles" -F "file=@testdata/articles_huge.ndjson" -F "resource=articles"
curl -X POST http://localhost:8080/v1/imports \
  -H "Idempotency-Key: test-comments" -F "file=@testdata/comments_huge.ndjson" -F "resource=comments"
```

### Test Results (`go test ./... -v -count=1`)

All 56 tests pass across 4 packages. Integration tests process all 45,000 testdata records through the full import pipeline:

```
=== RUN   TestHealthEndpoint
--- PASS: TestHealthEndpoint (0.00s)
=== RUN   TestMetricsEndpoint
--- PASS: TestMetricsEndpoint (0.00s)
=== RUN   TestGetImportStatus
--- PASS: TestGetImportStatus (0.00s)
=== RUN   TestGetImportStatus_NotFound
--- PASS: TestGetImportStatus_NotFound (0.00s)
=== RUN   TestGetImportErrors
--- PASS: TestGetImportErrors (0.00s)
=== RUN   TestGetImportErrors_CSV
--- PASS: TestGetImportErrors_CSV (0.00s)
=== RUN   TestExportStream_ValidationErrors
--- PASS: TestExportStream_ValidationErrors (0.00s)
=== RUN   TestImportValidation
--- PASS: TestImportValidation (0.00s)
=== RUN   TestIdempotencyKey
--- PASS: TestIdempotencyKey (0.00s)
=== RUN   TestCORSHeaders
--- PASS: TestCORSHeaders (0.00s)
=== RUN   TestImportWithWrongFileExtension
--- PASS: TestImportWithWrongFileExtension (0.00s)
=== RUN   TestCreateExport_Validation
--- PASS: TestCreateExport_Validation (0.00s)
=== RUN   TestGetExportStatus_NotFound
--- PASS: TestGetExportStatus_NotFound (0.00s)
=== RUN   TestGetImportErrors_EmptyErrors
--- PASS: TestGetImportErrors_EmptyErrors (0.00s)
ok  	github.com/bulk-import-export-api/internal/api	0.633s

=== RUN   TestMockUserRepository_BatchInsert
--- PASS: TestMockUserRepository_BatchInsert (0.00s)
=== RUN   TestMockUserRepository_DuplicateEmail
--- PASS: TestMockUserRepository_DuplicateEmail (0.00s)
=== RUN   TestMockUserRepository_Count
--- PASS: TestMockUserRepository_Count (0.00s)
=== RUN   TestMockJobRepository_PendingJobs
--- PASS: TestMockJobRepository_PendingJobs (0.00s)
=== RUN   TestMockJobRepository_MarkAsProcessing
--- PASS: TestMockJobRepository_MarkAsProcessing (0.00s)
=== RUN   TestMockJobRepository_ValidationErrors
--- PASS: TestMockJobRepository_ValidationErrors (0.00s)
=== RUN   TestMockJobRepository_IdempotencyKey
--- PASS: TestMockJobRepository_IdempotencyKey (0.00s)
ok  	github.com/bulk-import-export-api/internal/repository	1.615s

=== RUN   TestProcessImport_UsersCSV_HugeFile
    Users CSV: total=10000, successful=3233, failed=6767, errors=7086, batches=4, duration=16ms, rows/sec=617783
--- PASS: TestProcessImport_UsersCSV_HugeFile (0.02s)
=== RUN   TestProcessImport_UsersCSV_ContinueOnError
--- PASS: TestProcessImport_UsersCSV_ContinueOnError (0.01s)
=== RUN   TestProcessImport_UsersCSV_SpecificErrors
--- PASS: TestProcessImport_UsersCSV_SpecificErrors (0.01s)
=== RUN   TestProcessImport_UsersCSV_DuplicateEmails
    Found 19 duplicate email errors
--- PASS: TestProcessImport_UsersCSV_DuplicateEmails (0.01s)
=== RUN   TestProcessImport_UsersCSV_EmptyFile
--- PASS: TestProcessImport_UsersCSV_EmptyFile (0.00s)
=== RUN   TestProcessImport_UsersCSV_BatchInsertError
--- PASS: TestProcessImport_UsersCSV_BatchInsertError (0.00s)
=== RUN   TestProcessImport_ArticlesNDJSON_HugeFile
    Articles NDJSON: total=15000, successful=11155, failed=3845, errors=4083, batches=12, duration=162ms, rows/sec=92547
--- PASS: TestProcessImport_ArticlesNDJSON_HugeFile (0.16s)
=== RUN   TestProcessImport_ArticlesNDJSON_DraftWithPublishedAt
    Found 94 'draft with published_at' errors
--- PASS: TestProcessImport_ArticlesNDJSON_DraftWithPublishedAt (0.17s)
=== RUN   TestProcessImport_ArticlesNDJSON_MalformedJSON
--- PASS: TestProcessImport_ArticlesNDJSON_MalformedJSON (0.00s)
=== RUN   TestProcessImport_CommentsNDJSON_HugeFile
    Comments NDJSON: total=20000, successful=19105, failed=895, errors=930, batches=20, duration=104ms, rows/sec=192015
--- PASS: TestProcessImport_CommentsNDJSON_HugeFile (0.10s)
=== RUN   TestProcessImport_CommentsNDJSON_InvalidForeignKeys
    Found 457 invalid article_id errors
--- PASS: TestProcessImport_CommentsNDJSON_InvalidForeignKeys (0.09s)
=== RUN   TestProcessImport_CommentsNDJSON_MissingBody
    Found 250 missing body errors
--- PASS: TestProcessImport_CommentsNDJSON_MissingBody (0.11s)
=== RUN   TestJobService_GetJob
--- PASS: TestJobService_GetJob (0.00s)
=== RUN   TestJobService_GetJobWithMetrics
--- PASS: TestJobService_GetJobWithMetrics (0.00s)
=== RUN   TestExportService_StreamUsers
--- PASS: TestExportService_StreamUsers (0.00s)
=== RUN   TestMockImportService_CreateAndProcess
--- PASS: TestMockImportService_CreateAndProcess (0.00s)
=== RUN   TestMockExportService_GetCount
--- PASS: TestMockExportService_GetCount (0.00s)
=== RUN   TestMockJobService_IdempotencyKey
--- PASS: TestMockJobService_IdempotencyKey (0.00s)
ok  	github.com/bulk-import-export-api/internal/service	2.847s

=== RUN   TestValidateUser_RealCSVData
    Validated 100 records from real CSV: 65 failed
--- PASS: TestValidateUser_RealCSVData (0.00s)
=== RUN   TestValidateArticle_RealNDJSONData
    Validated 100 records from real NDJSON: 5 failed
--- PASS: TestValidateArticle_RealNDJSONData (0.00s)
=== RUN   TestValidateComment_RealNDJSONData
    Validated 100 records from real NDJSON: 9 failed
--- PASS: TestValidateComment_RealNDJSONData (0.00s)
=== RUN   TestValidateDuplicateEmails_AcrossRealData
    Detected 49 duplicate emails across 10000 records
--- PASS: TestValidateDuplicateEmails_AcrossRealData (0.02s)
=== RUN   TestValidateUser
--- PASS: TestValidateUser (0.00s)
=== RUN   TestValidateArticle
--- PASS: TestValidateArticle (0.00s)
=== RUN   TestValidateComment
--- PASS: TestValidateComment (0.00s)
=== RUN   TestDuplicateEmailDetection
--- PASS: TestDuplicateEmailDetection (0.00s)
=== RUN   TestDuplicateSlugDetection
--- PASS: TestDuplicateSlugDetection (0.00s)
=== RUN   TestValidationErrorMessages
--- PASS: TestValidationErrorMessages (0.00s)
=== RUN   TestKebabCaseValidation
--- PASS: TestKebabCaseValidation (0.00s)
=== RUN   TestCommentBodyWordBoundary
--- PASS: TestCommentBodyWordBoundary (0.00s)
=== RUN   TestValidateUser_UnicodeNames
--- PASS: TestValidateUser_UnicodeNames (0.00s)
=== RUN   TestValidateUser_EmptyActiveField
--- PASS: TestValidateUser_EmptyActiveField (0.00s)
=== RUN   TestValidateArticle_ForeignKeyValidation
--- PASS: TestValidateArticle_ForeignKeyValidation (0.00s)
ok  	github.com/bulk-import-export-api/internal/validation	1.131s
```

### Coverage (`go test ./... -cover`)

```
ok  github.com/bulk-import-export-api/internal/api          0.550s  coverage: 67.1% of statements
ok  github.com/bulk-import-export-api/internal/repository    1.139s  coverage:  0.0% of statements
ok  github.com/bulk-import-export-api/internal/service       2.364s  coverage: 46.8% of statements
ok  github.com/bulk-import-export-api/internal/validation    2.164s  coverage: 79.3% of statements
```

> Note: `internal/repository` shows 0% because it contains the real PostgreSQL implementations — tests use mock repositories that implement the same interfaces.

### Benchmark Results (`go test ./internal/service/ -bench=. -benchtime=3s`)

```
goos: darwin
goarch: arm64
pkg: github.com/bulk-import-export-api/internal/service
BenchmarkProcessImport_UsersCSV-12          	     273	  13640850 ns/op
BenchmarkProcessImport_ArticlesNDJSON-12    	      19	 167405200 ns/op
PASS
ok  	github.com/bulk-import-export-api/internal/service	12.312s
```

| Benchmark | Iterations | Time/op | Throughput |
|---|---|---|---|
| Users CSV (10K rows) | 273 | 13.6ms | ~734K rows/sec |
| Articles NDJSON (15K rows) | 19 | 167ms | ~90K rows/sec |

### Performance Testing Scripts

The project includes two scripts for end-to-end performance testing against a running server (Docker):

| Script | Tool | What it Tests |
|---|---|---|
| `scripts/benchmark.sh [N]` | bash + curl | End-to-end import + export throughput with N records (default: 10K) |
| `scripts/loadtest.js` | k6 | Concurrent HTTP load: 43 req/s across all 6 endpoints for 30s |

**`benchmark.sh`** generates test data, submits import jobs, polls for completion, tests export streaming, and reports throughput — all against the real PostgreSQL database. To prove the 1M target:

```bash
docker-compose up -d
./scripts/benchmark.sh 1000000
```

**`loadtest.js`** tests concurrent API load with ~1,240 total requests across all endpoint types simultaneously, verifying the API handles real-world concurrent access patterns.

## Design Decisions & Trade-offs

| Decision | Why | Trade-off |
|---|---|---|
| **Continue-on-error** | Bad records are logged and skipped; processing never stops on validation failures. The assignment requires processing large files where a few bad records shouldn't block the entire import. | Error accumulation: a file with 100% bad records still reads every line. Mitigated by structured error reporting so callers can batch-fix and re-import. |
| **Semaphore worker pool** | Bounded goroutines (`NumCPU * 4`, capped at 32) prevent OOM under high load. Uses a buffered channel as a semaphore per Dave Cheney's pattern. | Fixed pool size doesn't adapt to load. Could use an auto-scaling pool, but simplicity and predictability were prioritized for correctness. In production: make configurable via env var, add metrics on pool utilization. |
| **In-memory FK cache** | User/article IDs cached in a map for FK validation — avoids per-record database roundtrips, making validation O(1) per record. | Memory cost: ~100 bytes × N IDs. Capped at 100K IDs; beyond that FK validation is skipped with a warning. **Production alternative**: use a Bloom filter (probabilistic, ~1 byte/ID) or Redis SET for distributed FK validation across multiple API instances. |
| **PostgreSQL COPY protocol** | Batch inserts use `COPY ... FROM STDIN` for maximum throughput instead of multi-row INSERT. COPY avoids per-row SQL parsing overhead. Also used for error insertion (100K+ errors at high error rates). | COPY is all-or-nothing per batch: if one row violates a DB constraint, the entire 1,000-row batch fails. Mitigated by pre-validating all rows before batching. |
| **Streaming I/O (`csv.Reader` / `bufio.Scanner`)** | Files are parsed line-by-line, never loaded into memory. Guarantees O(1) memory regardless of file size (1K or 1M rows). | Cannot random-access or sort records. Not needed for this use case since validation and insert are sequential. |
| **Validation error flushing** | Errors flushed to DB every 1,000 entries instead of accumulating all in memory. At 1M records × 100% error rate, this caps memory at ~200KB instead of ~200MB. | Slightly more DB roundtrips (one COPY per 1K errors instead of one at end). Acceptable trade-off for bounded memory. |
| **Panic recovery per job** | Each goroutine has `defer recover()` so a single panicking job doesn't crash the worker pool or the process. | Recovered panics may leave partial state (some batches inserted, some not). The job is marked `failed` and the error is logged for investigation. |
| **CSV error export with `encoding/csv`** | Error reports use Go's `encoding/csv` Writer for proper field escaping. Prevents CSV injection where a malicious field value like `=CMD()` could execute in a spreadsheet. | Slightly more overhead than raw string formatting, but correctness and security matter more for exported reports. |
| **Idempotency via header** | `Idempotency-Key` header prevents duplicate job creation if the client retries a request. Lookup is O(1) via database index. | Keys are never expired in the current implementation. In production, would add a TTL-based cleanup job. |
| **No redundant indexes** | `UNIQUE` constraints on `email`, `slug`, and `idempotency_key` already create implicit btree indexes. Explicit duplicate indexes were removed to avoid write overhead. | N/A — purely beneficial; removes wasted disk space and write amplification during bulk inserts. |

## Production Scaling Notes

What would change for a production deployment handling 10M+ records across multiple API instances:

| Area | Current (Assignment) | Production |
|---|---|---|
| **FK validation** | In-memory `map[string]bool`, capped at 100K IDs, loaded once at import start | Bloom filter (~1 byte/ID, false positives acceptable) or Redis SET for distributed validation. At 10M users, a Bloom filter uses ~10MB vs ~1.6GB for a Go map. |
| **Worker pool** | Fixed `NumCPU * 4` goroutines, single-instance | Configurable via env var + autoscaling based on queue depth. Distribute across instances using Redis/SQS as job queue instead of polling PostgreSQL. |
| **File storage** | Local disk (`data/uploads/`), deleted after processing | S3/GCS with presigned upload URLs. Enables retry-from-file and multi-instance processing. |
| **Job queue** | PostgreSQL polling every 2s with `FOR UPDATE SKIP LOCKED` | Redis Stream or SQS for lower latency and distributed consumption. Current approach works to ~100 concurrent jobs before poll overhead matters. |
| **Error storage** | `job_errors` table, no TTL, grows unbounded | Partition by `created_at` month. Add cleanup cron deleting errors for completed jobs > 30 days old. At high error rates (1M records × 50% errors), this is 500K rows per job. |
| **Idempotency keys** | Never expired | Add TTL (e.g., 24h) with a background cleanup job. Current approach leaks ~100 bytes per key indefinitely. |
| **Export streaming** | `SELECT * ORDER BY created_at` full table scan | Add cursor-based pagination with `DECLARE CURSOR ... FETCH 1000` for true server-side streaming. Current approach works to ~5M rows before PostgreSQL sort buffer becomes an issue. |
| **Observability** | zerolog structured JSON to stdout | Add Prometheus metrics (`/metrics` endpoint) for import/export throughput, active workers, queue depth, error rates. Integrate with Grafana dashboards. |
| **Database** | Single PostgreSQL, all tables in one schema | Read replicas for exports, connection pooling (PgBouncer), table partitioning for comments (by `article_id` range) if > 100M rows. |

