package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New creates a new zerolog logger with structured output
func New() zerolog.Logger {
	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339

	// Get log level from environment
	level := os.Getenv("LOG_LEVEL")
	var logLevel zerolog.Level
	switch level {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	// Use pretty console output in development
	if os.Getenv("ENV") == "development" {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
			Level(logLevel).
			With().
			Timestamp().
			Caller().
			Str("service", "bulk-import-export-api").
			Logger()
	}

	// JSON output for production
	return zerolog.New(os.Stdout).
		Level(logLevel).
		With().
		Timestamp().
		Str("service", "bulk-import-export-api").
		Logger()
}
