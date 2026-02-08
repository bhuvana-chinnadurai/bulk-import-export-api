# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates wget curl

# Copy binary from builder
COPY --from=builder /app/server .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

# Copy seed data and scripts
COPY --from=builder /app/testdata/seed_*.csv /app/testdata/seed_*.ndjson /app/testdata/
COPY --from=builder /app/scripts/entrypoint.sh /app/scripts/seed.sh /app/scripts/
RUN chmod +x /app/scripts/*.sh

# Create data directory
RUN mkdir -p /app/data/uploads

# Expose port
EXPOSE 8080

# Set environment variables
ENV PORT=8080
ENV LOG_LEVEL=info
ENV MIGRATIONS_PATH=/app/migrations

# Start server and auto-seed sample data
CMD ["/app/scripts/entrypoint.sh"]
