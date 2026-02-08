#!/bin/sh
# Entrypoint: start the API server, then seed in the background.

# Run seed script in background (waits for server to be ready)
/app/scripts/seed.sh &

# Start the server (foreground — keeps container alive)
exec /app/server
