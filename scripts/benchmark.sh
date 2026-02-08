#!/bin/bash
# Performance Benchmark Script for Bulk Import/Export API
# Usage: ./scripts/benchmark.sh [num_records]

set -e

API_URL="${API_URL:-http://localhost:8080}"
NUM_RECORDS="${1:-10000}"  # Default 10k records
BATCH_SIZE=1000

echo "=============================================="
echo "  Bulk Import/Export API Performance Test"
echo "=============================================="
echo "API URL: $API_URL"
echo "Records: $NUM_RECORDS"
echo ""

# Check if API is running
echo "Checking API health..."
if ! curl -s "$API_URL/health" > /dev/null 2>&1; then
    echo "❌ API is not running. Please start with: docker-compose up -d"
    exit 1
fi
echo "✅ API is healthy"
echo ""

# Create test data directory
mkdir -p testdata

# ============================================
# Generate Test Data
# ============================================
echo "📝 Generating $NUM_RECORDS test users..."
START_GEN=$(date +%s.%N)

# Generate users CSV
echo "id,email,name,role,active,created_at" > testdata/users_benchmark.csv
for i in $(seq 1 $NUM_RECORDS); do
    uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    echo "$uuid,user${i}@benchmark.test,Benchmark User $i,admin,true,2024-01-01T00:00:00Z" >> testdata/users_benchmark.csv
done

END_GEN=$(date +%s.%N)
GEN_TIME=$(echo "$END_GEN - $START_GEN" | bc)
echo "✅ Generated in ${GEN_TIME}s"
echo ""

# ============================================
# Test Import Performance
# ============================================
echo "📥 Testing IMPORT performance ($NUM_RECORDS records)..."
START_IMPORT=$(date +%s.%N)

# Submit import job
RESPONSE=$(curl -s -X POST "$API_URL/v1/imports" \
    -H "Content-Type: multipart/form-data" \
    -H "Idempotency-Key: benchmark-$(date +%s)" \
    -F "resource=users" \
    -F "file=@testdata/users_benchmark.csv")

JOB_ID=$(echo "$RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$JOB_ID" ]; then
    echo "❌ Failed to create import job"
    echo "$RESPONSE"
    exit 1
fi

echo "   Job ID: $JOB_ID"

# Poll for completion
STATUS="pending"
while [ "$STATUS" != "completed" ] && [ "$STATUS" != "failed" ]; do
    sleep 1
    STATUS_RESPONSE=$(curl -s "$API_URL/v1/imports/$JOB_ID")
    STATUS=$(echo "$STATUS_RESPONSE" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
    PROCESSED=$(echo "$STATUS_RESPONSE" | grep -o '"processed_count":[0-9]*' | cut -d':' -f2)
    echo -ne "   Status: $STATUS | Processed: $PROCESSED/$NUM_RECORDS\r"
done
echo ""

END_IMPORT=$(date +%s.%N)
IMPORT_TIME=$(echo "$END_IMPORT - $START_IMPORT" | bc)

# Extract metrics from job response
SUCCESSFUL=$(echo "$STATUS_RESPONSE" | grep -o '"successful_count":[0-9]*' | cut -d':' -f2)
FAILED=$(echo "$STATUS_RESPONSE" | grep -o '"failed_count":[0-9]*' | cut -d':' -f2)
ROWS_PER_SEC=$(echo "$STATUS_RESPONSE" | grep -o '"rows_per_sec":[0-9.]*' | cut -d':' -f2)
DURATION_MS=$(echo "$STATUS_RESPONSE" | grep -o '"duration_ms":[0-9]*' | cut -d':' -f2)

echo ""
echo "📊 IMPORT Results:"
echo "   ✅ Successful: $SUCCESSFUL"
echo "   ❌ Failed: $FAILED"
echo "   ⏱️  Duration: ${DURATION_MS}ms"
echo "   🚀 Throughput: $ROWS_PER_SEC rows/sec"
echo ""

# ============================================
# Test Export Performance
# ============================================
echo "📤 Testing EXPORT performance (streaming NDJSON)..."
START_EXPORT=$(date +%s.%N)

# Stream export and count lines
EXPORT_COUNT=$(curl -s "$API_URL/v1/exports?resource=users&format=ndjson" | wc -l | tr -d ' ')

END_EXPORT=$(date +%s.%N)
EXPORT_TIME=$(echo "$END_EXPORT - $START_EXPORT" | bc)
EXPORT_ROWS_PER_SEC=$(echo "scale=2; $EXPORT_COUNT / $EXPORT_TIME" | bc)

echo ""
echo "📊 EXPORT Results:"
echo "   ✅ Records exported: $EXPORT_COUNT"
echo "   ⏱️  Duration: ${EXPORT_TIME}s"
echo "   🚀 Throughput: $EXPORT_ROWS_PER_SEC rows/sec"
echo ""

# ============================================
# Summary
# ============================================
echo "=============================================="
echo "                  SUMMARY"
echo "=============================================="
echo ""
echo "📥 Import: $ROWS_PER_SEC rows/sec"
echo "📤 Export: $EXPORT_ROWS_PER_SEC rows/sec"
echo ""
echo "Assignment requirement: 5k rows/sec for NDJSON export"
echo ""

if (( $(echo "$EXPORT_ROWS_PER_SEC > 5000" | bc -l) )); then
    echo "✅ PASSED: Export exceeds 5k rows/sec requirement!"
else
    echo "⚠️  Export below 5k rows/sec (may be limited by network/CPU)"
fi

# Cleanup
rm -f testdata/users_benchmark.csv

echo ""
echo "Done!"
