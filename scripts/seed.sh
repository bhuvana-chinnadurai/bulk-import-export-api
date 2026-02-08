#!/bin/sh
# Seed the database with sample data via the import API.
# Runs in the background after the server starts.

set -e

API_URL="http://localhost:8080"
SEED_DIR="/app/testdata"

# Wait for API to be ready
for i in $(seq 1 30); do
  if curl -sf "$API_URL/health" > /dev/null 2>&1; then
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "[seed] API not ready after 30s, skipping."
    exit 0
  fi
  sleep 1
done

# Skip if data already exists
USERS=$(curl -sf "$API_URL/metrics" 2>/dev/null | grep -o '"users":[0-9]*' | grep -o '[0-9]*' || echo "0")
if [ "$USERS" -gt 0 ] 2>/dev/null; then
  echo "[seed] Database already seeded ($USERS users), skipping."
  exit 0
fi

echo "[seed] Seeding database with sample data..."

# Import and wait for completion before starting next (FK dependencies)
import_and_wait() {
  RESOURCE="$1"
  FILE="$2"

  if [ ! -f "$SEED_DIR/$FILE" ]; then
    echo "[seed]   $FILE not found, skipping."
    return
  fi

  echo "[seed]   Importing $RESOURCE..."
  RESPONSE=$(curl -sf -X POST "$API_URL/v1/imports" \
    -H "Idempotency-Key: seed-$RESOURCE" \
    -F "resource=$RESOURCE" \
    -F "file=@$SEED_DIR/$FILE" 2>&1) || { echo "[seed]   $RESOURCE: request failed"; return; }

  JOB_ID=$(echo "$RESPONSE" | grep -o '"job_id":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [ -z "$JOB_ID" ]; then
    echo "[seed]   $RESOURCE: no job_id in response"
    return
  fi

  # Poll until complete
  for i in $(seq 1 30); do
    STATUS=$(curl -sf "$API_URL/v1/imports/$JOB_ID" 2>/dev/null | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
      echo "[seed]   $RESOURCE: $STATUS"
      return
    fi
    sleep 1
  done
  echo "[seed]   $RESOURCE: timeout"
}

# Order matters: users → articles (FK: author_id) → comments (FK: article_id, user_id)
import_and_wait "users" "seed_users.csv"
import_and_wait "articles" "seed_articles.ndjson"
import_and_wait "comments" "seed_comments.ndjson"

echo "[seed] Done. Database counts:"
curl -sf "$API_URL/metrics" 2>/dev/null || echo "[seed] Could not fetch metrics"
